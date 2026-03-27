local key = KEYS[1]
local cntKey = key .. ":cnt"
local expectedCode = ARGV[1]

local cnt = tonumber(redis.call("get",cntKey) or "-1")
local code = redis.call("get",key)

if not code or cnt <= 0 then
    return -1 ---  验证码不存在或尝试次数过多
end

if code == expectedCode then --- 验证成功
    local ttl = redis.call("ttl",cntKey)
    redis.call("set",cntKey,-1) --- 把cnt可尝试次数设为-1
    if ttl > 0 then
        redis.call("expire",cntKey,ttl) --- 让尝试次数和验证码一起过期
    end
    return 0
else
    redis.call("decr",cntKey) --- 验证码存在但不正确,可尝试次数减1
    return -2
end