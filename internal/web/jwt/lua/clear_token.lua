local refreshKey = KEYS[1]
local logoutKey = KEYS[2]
local ttl = tonumber(ARGV[1])

redis.call("DEL", refreshKey)
redis.call("SET", logoutKey, "logout", "EX", ttl)

return 1