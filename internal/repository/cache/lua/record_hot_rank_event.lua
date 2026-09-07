if redis.call('EXISTS', KEYS[2]) == 1 then
    return 0
end

redis.call('ZINCRBY', KEYS[1], tonumber(ARGV[1]), ARGV[2])
redis.call('PEXPIREAT', KEYS[1], tonumber(ARGV[3]))
redis.call('SET', KEYS[2], '1', 'PX', tonumber(ARGV[4]))
return 1
