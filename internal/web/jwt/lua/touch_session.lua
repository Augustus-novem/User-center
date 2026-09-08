local logoutKey = KEYS[1]
local refreshKey = KEYS[2]
local absoluteKey = KEYS[3]
local idleTTL = tonumber(ARGV[1])

if redis.call("EXISTS", logoutKey) == 1 then
    return 0
end
if redis.call("EXISTS", refreshKey) == 0 then
    return 0
end
local absoluteTTL = redis.call("PTTL", absoluteKey)
if absoluteTTL <= 0 then
    return 0
end
redis.call("PEXPIRE", refreshKey, math.min(idleTTL, absoluteTTL))
return 1
