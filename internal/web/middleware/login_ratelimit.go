package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// LoginRateLimiter 登录接口专用限流器
type LoginRateLimiter struct {
	client redis.Cmdable
	prefix string
}

func NewLoginRateLimiter(client redis.Cmdable) *LoginRateLimiter {
	return &LoginRateLimiter{
		client: client,
		prefix: "login:ratelimit",
	}
}

// Build 构建登录限流中间件
// 策略：同一 IP 5分钟内最多尝试 5 次
func (l *LoginRateLimiter) Build() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ip := ctx.ClientIP()
		key := fmt.Sprintf("%s:ip:%s", l.prefix, ip)
		
		allowed, remaining, err := l.checkLimit(ctx.Request.Context(), key, 5, 5*time.Minute)
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"code": 500,
				"msg":  "系统错误",
			})
			return
		}
		
		// 设置响应头，告知客户端剩余次数
		ctx.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
		
		if !allowed {
			ctx.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code": 429,
				"msg":  "登录尝试次数过多，请5分钟后再试",
			})
			return
		}
		
		ctx.Next()
	}
}

// BuildWithAccount 基于账号的限流（防止暴力破解）
// 策略：同一账号 10分钟内最多尝试 3 次
func (l *LoginRateLimiter) BuildWithAccount() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// 从请求体中提取账号信息
		type LoginReq struct {
			Email string `json:"email"`
			Phone string `json:"phone"`
		}
		
		var req LoginReq
		if err := ctx.ShouldBindJSON(&req); err != nil {
			ctx.Next()
			return
		}
		
		// 重新设置请求体，供后续处理器使用
		ctx.Set("login_request", req)
		
		account := req.Email
		if account == "" {
			account = req.Phone
		}
		
		if account == "" {
			ctx.Next()
			return
		}
		
		key := fmt.Sprintf("%s:account:%s", l.prefix, account)
		allowed, remaining, err := l.checkLimit(ctx.Request.Context(), key, 3, 10*time.Minute)
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"code": 500,
				"msg":  "系统错误",
			})
			return
		}
		
		ctx.Header("X-RateLimit-Account-Remaining", fmt.Sprintf("%d", remaining))
		
		if !allowed {
			ctx.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code": 429,
				"msg":  "该账号登录尝试次数过多，请10分钟后再试",
			})
			return
		}
		
		ctx.Next()
	}
}

func (l *LoginRateLimiter) checkLimit(ctx context.Context, key string, limit int64, window time.Duration) (bool, int64, error) {
	now := time.Now().UnixMilli()
	windowStart := now - window.Milliseconds()
	
	pipe := l.client.Pipeline()
	
	// 删除窗口外的记录
	pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", windowStart))
	
	// 统计当前窗口内的请求数
	countCmd := pipe.ZCount(ctx, key, fmt.Sprintf("%d", windowStart), fmt.Sprintf("%d", now))
	
	// 添加当前请求
	pipe.ZAdd(ctx, key, redis.Z{
		Score:  float64(now),
		Member: fmt.Sprintf("%d", now),
	})
	
	// 设置过期时间
	pipe.Expire(ctx, key, window+time.Minute)
	
	_, err := pipe.Exec(ctx)
	if err != nil {
		return false, 0, err
	}
	
	count := countCmd.Val()
	remaining := limit - count
	if remaining < 0 {
		remaining = 0
	}
	
	return count < limit, remaining, nil
}
