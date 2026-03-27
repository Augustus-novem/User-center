local key = KEYS[1]
local cntKey = key..":cnt"
local val = ARGV[1]
local ttl = tonumber(redis.call("ttl",key))

if ttl == -1 then
    return -2 ---系统异常有一个永不过期的key
elseif ttl==-2 or ttl < 540 then
    redis.call("setex",key,600,val)
    redis.call("setex",cntKey,600,3)
    return 0
else
    return -1 --- ttl>540也就是还没有一分钟
end