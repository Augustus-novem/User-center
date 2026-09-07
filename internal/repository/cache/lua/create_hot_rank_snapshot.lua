if redis.call('EXISTS', KEYS[2]) == 0 then
    local command = {'ZUNIONSTORE', KEYS[1], #KEYS - 2}
    for i = 3, #KEYS do
        table.insert(command, KEYS[i])
    end
    table.insert(command, 'AGGREGATE')
    table.insert(command, 'SUM')
    redis.call(unpack(command))
    redis.call('PEXPIRE', KEYS[1], tonumber(ARGV[1]))
    redis.call('SET', KEYS[2], '1', 'PX', tonumber(ARGV[1]))
end
return 1
