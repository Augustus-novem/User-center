-- 带版本号的热榜更新脚本
-- KEYS[1]: 日榜 key
-- KEYS[2]: 月榜 key
-- KEYS[3]: 版本 key
-- ARGV[1]: member (用户ID)
-- ARGV[2]: 日榜分数
-- ARGV[3]: 月榜增量
-- ARGV[4]: 期望版本号
-- ARGV[5]: 日榜过期时间戳
-- ARGV[6]: 月榜过期时间戳

local dailyKey = KEYS[1]
local monthlyKey = KEYS[2]
local versionKey = KEYS[3]
local member = ARGV[1]
local dailyScore = tonumber(ARGV[2])
local monthlyDelta = tonumber(ARGV[3])
local expectedVersion = tonumber(ARGV[4])
local dailyExpire = tonumber(ARGV[5])
local monthlyExpire = tonumber(ARGV[6])

-- 获取当前版本号
local currentVersion = redis.call('GET', versionKey)
if currentVersion == false then
    currentVersion = 0
else
    currentVersion = tonumber(currentVersion)
end

-- 版本号检查
if currentVersion ~= expectedVersion then
    return 0  -- 版本冲突
end

-- 更新日榜
redis.call('ZADD', dailyKey, dailyScore, member)
redis.call('EXPIREAT', dailyKey, dailyExpire)

-- 更新月榜
redis.call('ZINCRBY', monthlyKey, monthlyDelta, member)
redis.call('EXPIREAT', monthlyKey, monthlyExpire)

-- 更新版本号
local newVersion = currentVersion + 1
redis.call('SET', versionKey, newVersion, 'EX', 86400 * 2)  -- 2天过期

return 1  -- 更新成功
