-- wrk 压测脚本 - 登录接口

-- 初始化
wrk.method = "POST"
wrk.headers["Content-Type"] = "application/json"

-- 测试账号列表
local accounts = {
    {email = "test1@example.com", password = "Test123!@#"},
    {email = "test2@example.com", password = "Test123!@#"},
    {email = "test3@example.com", password = "Test123!@#"},
    {email = "test4@example.com", password = "Test123!@#"},
    {email = "test5@example.com", password = "Test123!@#"},
}

-- 计数器
local counter = 0

-- 每个请求调用
function request()
    counter = counter + 1
    local account = accounts[(counter % #accounts) + 1]
    
    local body = string.format(
        '{"email":"%s","password":"%s"}',
        account.email,
        account.password
    )
    
    return wrk.format(nil, nil, nil, body)
end

-- 响应处理
function response(status, headers, body)
    if status ~= 200 then
        print("Error: " .. status .. " - " .. body)
    end
end

-- 完成后统计
function done(summary, latency, requests)
    io.write("------------------------------\n")
    io.write("压测结果统计:\n")
    io.write("------------------------------\n")
    io.write(string.format("总请求数: %d\n", summary.requests))
    io.write(string.format("总耗时: %.2fs\n", summary.duration / 1000000))
    io.write(string.format("平均 QPS: %.2f\n", summary.requests / (summary.duration / 1000000)))
    io.write(string.format("平均延迟: %.2fms\n", latency.mean / 1000))
    io.write(string.format("P50 延迟: %.2fms\n", latency:percentile(50) / 1000))
    io.write(string.format("P90 延迟: %.2fms\n", latency:percentile(90) / 1000))
    io.write(string.format("P99 延迟: %.2fms\n", latency:percentile(99) / 1000))
    io.write(string.format("最大延迟: %.2fms\n", latency.max / 1000))
    io.write(string.format("错误数: %d\n", summary.errors.connect + summary.errors.read + summary.errors.write + summary.errors.status + summary.errors.timeout))
    io.write("------------------------------\n")
end
