package ratelimit

import (
	_ "embed"
	"fmt"
	"net/http"
	"time"
	"user-center/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type Builder struct {
	prefix   string
	cmd      redis.Cmdable
	interval time.Duration
	// 阈值
	rate   int
	logger logger.Logger
}

//go:embed slide_window.lua
var luaScript string

func NewBuilder(cmd redis.Cmdable, interval time.Duration, rate int, l logger.Logger) *Builder {
	if l == nil {
		l = logger.NewNoOpLogger()
	}
	return &Builder{
		cmd:      cmd,
		prefix:   "ip-limiter",
		interval: interval,
		rate:     rate,
		logger:   l,
	}
}

func (b *Builder) Prefix(prefix string) *Builder {
	b.prefix = prefix
	return b
}

func (b *Builder) Build() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		limited, err := b.limit(ctx)
		if err != nil {
			b.logger.Error("限流检查失败",
				logger.Field{Key: "path", Value: ctx.Request.URL.Path},
				logger.Field{Key: "ip", Value: ctx.ClientIP()},
				logger.Error(err))
			ctx.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		if limited {
			b.logger.Warn("请求被限流",
				logger.Field{Key: "path", Value: ctx.Request.URL.Path},
				logger.Field{Key: "ip", Value: ctx.ClientIP()},
			)
			ctx.AbortWithStatus(http.StatusTooManyRequests)
			return
		}
		ctx.Next()
	}
}

func (b *Builder) limit(ctx *gin.Context) (bool, error) {
	key := fmt.Sprintf("%s:%s", b.prefix, ctx.ClientIP())
	return b.cmd.Eval(ctx, luaScript, []string{key},
		b.interval.Milliseconds(), b.rate, time.Now().UnixMilli(), uuid.New().String()).Bool()
}
