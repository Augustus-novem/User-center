local refreshKey = KEYS[1]
local absoluteKey = KEYS[2]
local logoutKey = KEYS[3]
local oldJTI = ARGV[1]
local newJTI = ARGV[2]
local idleTTL = tonumber(ARGV[3])

if redis.call("EXISTS", logoutKey) == 1 then
    return 0
end
if redis.call("GET", refreshKey) ~= oldJTI then
    return 0
end
local absoluteTTL = redis.call("PTTL", absoluteKey)
if absoluteTTL <= 0 then
    return 0
end
local ttl = math.min(idleTTL, absoluteTTL)
redis.call("SET", refreshKey, newJTI, "PX", ttl)
return 1
