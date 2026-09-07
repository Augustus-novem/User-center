-- KEYS[1] feed inbox zset
-- ARGV[1] score (note_id)
-- ARGV[2] member (note_id)
-- ARGV[3] max items
redis.call('ZADD', KEYS[1], ARGV[1], ARGV[2])
local max = tonumber(ARGV[3])
if max ~= nil and max > 0 then
	local size = redis.call('ZCARD', KEYS[1])
	if size > max then
		redis.call('ZREMRANGEBYRANK', KEYS[1], 0, size - max - 1)
	end
end
return 1
