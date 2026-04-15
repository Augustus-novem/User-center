package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// RateLimiter 接口限流中间件
type RateLimiter struct {
	client   redis.Cmdable
	prefix   string
	interval time.Duration
	limit    int64
}

func NewRateLimiter(client redis.Cmdable, prefix string, interval time.Duration, limit int64) *RateLimiter {
	return &RateLimiter{
		client:   client,
		prefix:   prefix,
		interval: interval,
		limit:    limit,
	}
}

// Build 构建限流中间件
func (r *RateLimiter) Build() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// 使用 IP 作为限流键
		key := fmt.Sprintf("%s:%s", r.prefix, ctx.ClientIP())
		
		allowed, err := r.allow(ctx.Request.Context(), key)
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"code": 500,
				"msg":  "系统错误",
			})
			return
		}
		
		if !allowed {
			ctx.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code": 429,
				"msg":  "请求过于频繁，请稍后再试",
			})
			return
		}
		
		ctx.Next()
	}
}

// BuildWithUserID 基于用户 ID 的限流中间件
func (r *RateLimiter) BuildWithUserID() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		val, exists := ctx.Get("user")
		if !exists {
			ctx.Next()
			return
		}
		
		userID := fmt.Sprintf("%v", val)
		key := fmt.Sprintf("%s:user:%s", r.prefix, userID)
		
		allowed, err := r.allow(ctx.Request.Context(), key)
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"code": 500,
				"msg":  "系统错误",
			})
			return
		}
		
		if !allowed {
			ctx.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code": 429,
				"msg":  "操作过于频繁，请稍后再试",
			})
			return
		}
		
		ctx.Next()
	}
}

func (r *RateLimiter) allow(ctx context.Context, key string) (bool, error) {
	// 使用滑动窗口算法
	now := time.Now().UnixMilli()
	windowStart := now - r.interval.Milliseconds()
	
	pipe := r.client.Pipeline()
	
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
	pipe.Expire(ctx, key, r.interval+time.Minute)
	
	_, err := pipe.Exec(ctx)
	if err != nil {
		return false, err
	}
	
	count := countCmd.Val()
	return count < r.limit, nil
}
