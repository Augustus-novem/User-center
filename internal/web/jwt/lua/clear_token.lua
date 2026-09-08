local refreshKey = KEYS[1]
local absoluteKey = KEYS[2]
local logoutKey = KEYS[3]
local ttl = tonumber(ARGV[1])

redis.call("DEL", refreshKey, absoluteKey)
redis.call("SET", logoutKey, "logout", "EX", ttl)
return 1
