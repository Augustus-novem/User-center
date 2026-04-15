package examples

import (
	"time"
	"user-center/internal/web"
	"user-center/internal/web/middleware"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// 限流使用示例
func SetupRateLimitExample(router *gin.Engine, redisClient redis.Cmdable) {
	// 1. 全局 IP 限流：1分钟内最多 100 次请求
	globalLimiter := middleware.NewRateLimiter(
		redisClient,
		"global-limiter",
		time.Minute,
		100,
	)
	router.Use(globalLimiter.Build())

	// 2. 登录接口限流
	loginLimiter := middleware.NewLoginRateLimiter(redisClient)
	
	// 登录路由组
	authGroup := router.Group("/auth")
	{
		// IP 限流：5分钟内最多 5 次
		authGroup.POST("/login", loginLimiter.Build(), func(ctx *gin.Context) {
			// 登录逻辑
			web.JSONOK(ctx, "登录成功", nil)
		})
		
		// 账号限流：10分钟内最多 3 次
		authGroup.POST("/login-strict", 
			loginLimiter.Build(),
			loginLimiter.BuildWithAccount(),
			func(ctx *gin.Context) {
				// 登录逻辑
				web.JSONOK(ctx, "登录成功", nil)
			},
		)
	}

	// 3. 用户级别限流：针对已登录用户
	userGroup := router.Group("/api")
	userGroup.Use(globalLimiter.BuildWithUserID()) // 每个用户独立限流
	{
		userGroup.POST("/action", func(ctx *gin.Context) {
			web.JSONOK(ctx, "操作成功", nil)
		})
	}

	// 4. 特定接口自定义限流
	apiLimiter := middleware.NewRateLimiter(
		redisClient,
		"api-send-sms",
		5*time.Minute, // 5分钟
		3,             // 最多3次
	)
	router.POST("/sms/send", apiLimiter.Build(), func(ctx *gin.Context) {
		web.JSONOK(ctx, "短信发送成功", nil)
	})
}
