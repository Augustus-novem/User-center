local refreshKey = KEYS[1]
local absoluteKey = KEYS[2]
local logoutKey = KEYS[3]
local jti = ARGV[1]
local idleTTL = tonumber(ARGV[2])
local absoluteTTL = tonumber(ARGV[3])
local absoluteDeadline = ARGV[4]

redis.call("DEL", logoutKey)
redis.call("SET", absoluteKey, absoluteDeadline, "PX", absoluteTTL)
redis.call("SET", refreshKey, jti, "PX", math.min(idleTTL, absoluteTTL))
return 1
