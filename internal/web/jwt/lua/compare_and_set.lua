local key = KEYS[1]
local old = ARGV[1]
local new = ARGV[2]
local ttl = tonumber(ARGV[3])

local cur = redis.call("GET", key)
if not cur then
    return 0
end

if cur ~= old then
    return 0
end

redis.call("SET", key, new, "EX", ttl)
return 1