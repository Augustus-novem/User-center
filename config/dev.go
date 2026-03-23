//go:build !k8s

package config

var Config = UserConfig{
	DB: DBConfig{
		DSN: "root:root@tcp(localhost:13316)/user_center",
	},
	Redis: RedisConfig{
		Addr:     "localhost:6379",
		Password: "",
		DB:       1,
	},
}
