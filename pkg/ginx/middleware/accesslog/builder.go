package accesslog

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type AccessLog struct {
	Method     string `json:"method"`
	Path       string `json:"path"`
	Route      string `json:"route"`
	ClientIP   string `json:"client_ip"`
	UserAgent  string `json:"user_agent"`
	ReqBody    string `json:"req_body,omitempty"`
	RespBody   string `json:"resp_body,omitempty"`
	Duration   string `json:"duration"`
	StatusCode int    `json:"status_code"`
	Errors     string `json:"errors,omitempty"`
}

type bodyLogRule struct {
	method string
	path   string
}

type BodyMasker func(c *gin.Context, body []byte) []byte

type MiddlewareBuilder struct {
	logFunc func(ctx context.Context, al AccessLog)

	bodyAllowList  []bodyLogRule
	reqBodyMasker  BodyMasker
	respBodyMasker BodyMasker
}

func NewMiddlewareBuilder(fn func(ctx context.Context, al AccessLog)) *MiddlewareBuilder {
	return &MiddlewareBuilder{
		logFunc: fn,
	}
}

// AllowBodyFor 只有命中的接口才会打印 req/resp body。
// method 传空字符串表示不限制 method。
// path 使用请求实际路径精确匹配，比如 "/user/login"。
func (b *MiddlewareBuilder) AllowBodyFor(method, path string) *MiddlewareBuilder {
	b.bodyAllowList = append(b.bodyAllowList, bodyLogRule{
		method: method,
		path:   path,
	})
	return b
}

// SetReqBodyMasker 设置请求体脱敏函数。
func (b *MiddlewareBuilder) SetReqBodyMasker(masker BodyMasker) *MiddlewareBuilder {
	b.reqBodyMasker = masker
	return b
}

// SetRespBodyMasker 设置响应体脱敏函数。
func (b *MiddlewareBuilder) SetRespBodyMasker(masker BodyMasker) *MiddlewareBuilder {
	b.respBodyMasker = masker
	return b
}

// SetBodyMasker 给 req/resp 共用同一个脱敏函数。
func (b *MiddlewareBuilder) SetBodyMasker(masker BodyMasker) *MiddlewareBuilder {
	b.reqBodyMasker = masker
	b.respBodyMasker = masker
	return b
}

func (b *MiddlewareBuilder) Build() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		al := AccessLog{
			Method:    c.Request.Method,
			Path:      c.Request.URL.Path,
			ClientIP:  c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
		}
		logBody := b.shouldLogBody(c)

		var rw *responseWriter

		if logBody && c.Request != nil && c.Request.Body != nil {
			rawReqBody, err := c.GetRawData()
			if err == nil {
				if b.reqBodyMasker != nil {
					al.ReqBody = string(b.reqBodyMasker(c, rawReqBody))
				} else {
					al.ReqBody = string(rawReqBody)
				}
				c.Request.Body = io.NopCloser(bytes.NewBuffer(rawReqBody))
				c.Request.ContentLength = int64(len(rawReqBody))
			}
		}

		if logBody {
			rw = &responseWriter{
				ResponseWriter: c.Writer,
			}
			c.Writer = rw
		}

		defer func() {
			al.Duration = time.Since(start).String()
			al.StatusCode = c.Writer.Status()
			al.Route = c.FullPath()

			if len(c.Errors) > 0 {
				al.Errors = c.Errors.String()
			}

			if logBody && rw != nil {
				respBody := rw.body.Bytes()
				if len(respBody) > 0 {
					if b.respBodyMasker != nil {
						al.RespBody = string(b.respBodyMasker(c, respBody))
					} else {
						al.RespBody = string(respBody)
					}
				}
			}

			if b.logFunc != nil {
				b.logFunc(c.Request.Context(), al)
			}
		}()

		c.Next()
	}
}

func (b *MiddlewareBuilder) shouldLogBody(c *gin.Context) bool {
	if len(b.bodyAllowList) == 0 {
		return false
	}
	method := strings.ToUpper(c.Request.Method)
	path := c.Request.URL.Path

	for _, rule := range b.bodyAllowList {
		methodMatched := rule.method == "" || rule.method == method
		pathMatched := rule.path == path
		if methodMatched && pathMatched {
			return true
		}
	}
	return false
}

type responseWriter struct {
	gin.ResponseWriter
	body bytes.Buffer
}

func (r *responseWriter) WriteHeader(statusCode int) {
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *responseWriter) Write(data []byte) (int, error) {
	// 只记录文本类响应，避免把二进制乱写进日志
	if isTextLikeContentType(r.Header().Get("Content-Type")) {
		_, _ = r.body.Write(data)
	}
	return r.ResponseWriter.Write(data)
}

func (r *responseWriter) WriteString(data string) (int, error) {
	if isTextLikeContentType(r.Header().Get("Content-Type")) {
		_, _ = r.body.WriteString(data)
	}
	return r.ResponseWriter.WriteString(data)
}

func isTextLikeContentType(contentType string) bool {
	contentType = strings.ToLower(contentType)
	if contentType == "" {
		return true
	}
	return strings.Contains(contentType, "application/json") ||
		strings.Contains(contentType, "text/") ||
		strings.Contains(contentType, "application/xml") ||
		strings.Contains(contentType, "application/x-www-form-urlencoded")
}

// 你后面如果要扩展成前缀匹配、正则匹配，可以从这里继续增强。
func MatchMethodAndPath(method, path string) func(*http.Request) bool {
	method = strings.ToUpper(strings.TrimSpace(method))
	path = strings.TrimSpace(path)
	return func(r *http.Request) bool {
		if r == nil {
			return false
		}
		if method != "" && strings.ToUpper(r.Method) != method {
			return false
		}
		return r.URL.Path == path
	}
}
