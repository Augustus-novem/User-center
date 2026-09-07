if redis.call('EXISTS', KEYS[2]) == 0 then
    redis.call('ZUNIONSTORE', KEYS[1], #KEYS - 2, unpack(KEYS, 3), 'AGGREGATE', 'SUM')
    redis.call('PEXPIRE', KEYS[1], tonumber(ARGV[1]))
    redis.call('SET', KEYS[2], '1', 'PX', tonumber(ARGV[1]))
end
return 1
