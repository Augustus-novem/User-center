# 简单的测试脚本 - 不依赖 Kafka

Write-Host "==========================================" -ForegroundColor Cyan
Write-Host "  用户中心功能测试" -ForegroundColor Cyan
Write-Host "==========================================" -ForegroundColor Cyan
Write-Host ""

# 检查服务是否运行
Write-Host "检查服务状态..." -ForegroundColor Yellow
try {
    $response = Invoke-RestMethod -Uri "http://localhost:8081/rank/daily" -Method Get -ErrorAction Stop
    Write-Host "OK 服务运行正常" -ForegroundColor Green
}
catch {
    Write-Host "ERROR 服务未运行，请先启动服务" -ForegroundColor Red
    Write-Host "  运行: go run cmd/notification-service/main.go"
    exit 1
}
Write-Host ""

# 1. 测试限流
Write-Host "==========================================" -ForegroundColor Cyan
Write-Host "1. 测试接口限流" -ForegroundColor Cyan
Write-Host "==========================================" -ForegroundColor Cyan

Write-Host "发送 10 个并发请求..."
$jobs = @()
for ($i = 1; $i -le 10; $i++) {
    $jobs += Start-Job -ScriptBlock {
        try {
            Invoke-RestMethod -Uri "http://localhost:8081/rank/daily" -Method Get -ErrorAction Stop
            return "success"
        }
        catch {
            return "failed"
        }
    }
}

$results = $jobs | Wait-Job | Receive-Job
$jobs | Remove-Job

$successCount = ($results | Where-Object { $_ -eq "success" }).Count
$failedCount = ($results | Where-Object { $_ -eq "failed" }).Count

Write-Host "结果: 成功 $successCount, 失败 $failedCount"
if ($failedCount -gt 0) {
    Write-Host "OK 限流功能正常（部分请求被限制）" -ForegroundColor Green
}
else {
    Write-Host "WARNING 可能未触发限流（请求数较少）" -ForegroundColor Yellow
}
Write-Host ""

# 2. 测试登录限流
Write-Host "==========================================" -ForegroundColor Cyan
Write-Host "2. 测试登录限流" -ForegroundColor Cyan
Write-Host "==========================================" -ForegroundColor Cyan

Write-Host "连续发送 6 次登录请求..."
$limitTriggered = $false

for ($i = 1; $i -le 6; $i++) {
    try {
        $body = @{
            email = "test@example.com"
            password = "Test123!@#"
        } | ConvertTo-Json
        
        $response = Invoke-WebRequest -Uri "http://localhost:8081/user/login" `
            -Method Post `
            -ContentType "application/json" `
            -Body $body `
            -ErrorAction Stop
        
        Write-Host "  第 $i 次: $($response.StatusCode)" -ForegroundColor Gray
    }
    catch {
        $statusCode = $_.Exception.Response.StatusCode.value__
        if ($statusCode -eq 429) {
            Write-Host "  第 $i 次: 429 (被限流) OK" -ForegroundColor Green
            $limitTriggered = $true
        }
        else {
            Write-Host "  第 $i 次: $statusCode" -ForegroundColor Gray
        }
    }
    Start-Sleep -Milliseconds 500
}

if ($limitTriggered) {
    Write-Host "OK 登录限流功能正常" -ForegroundColor Green
}
else {
    Write-Host "WARNING 未触发登录限流（可能需要更多请求）" -ForegroundColor Yellow
}
Write-Host ""

# 3. 测试 Redis 连接
Write-Host "==========================================" -ForegroundColor Cyan
Write-Host "3. 检查 Redis 键" -ForegroundColor Cyan
Write-Host "==========================================" -ForegroundColor Cyan

try {
    $limiterKeys = redis-cli --scan --pattern "*limiter*" 2>$null
    $limiterCount = ($limiterKeys | Measure-Object).Count
    Write-Host "限流键数量: $limiterCount"
    
    if ($limiterCount -gt 0) {
        Write-Host "OK Redis 限流键已创建" -ForegroundColor Green
        Write-Host "示例键:"
        $limiterKeys | Select-Object -First 3 | ForEach-Object { Write-Host "  $_" -ForegroundColor Gray }
    }
    else {
        Write-Host "WARNING 未找到限流键" -ForegroundColor Yellow
    }
}
catch {
    Write-Host "ERROR 无法连接 Redis" -ForegroundColor Red
}
Write-Host ""

# 4. 测试热榜并发
Write-Host "==========================================" -ForegroundColor Cyan
Write-Host "4. 测试热榜缓存一致性" -ForegroundColor Cyan
Write-Host "==========================================" -ForegroundColor Cyan

if (Test-Path "test/concurrent_rank_update.go") {
    Write-Host "运行并发测试..."
    go run test/concurrent_rank_update.go
}
else {
    Write-Host "WARNING 测试文件不存在，跳过" -ForegroundColor Yellow
}
Write-Host ""

# 5. 运行单元测试
Write-Host "==========================================" -ForegroundColor Cyan
Write-Host "5. 运行单元测试" -ForegroundColor Cyan
Write-Host "==========================================" -ForegroundColor Cyan

Write-Host "测试 Worker 模块..."
$workerTest = go test ./internal/worker -v -count=1 2>&1
if ($LASTEXITCODE -eq 0) {
    Write-Host "OK Worker 测试通过" -ForegroundColor Green
}
else {
    Write-Host "ERROR Worker 测试失败" -ForegroundColor Red
}
Write-Host ""

Write-Host "测试 Cache 模块..."
$cacheTest = go test ./internal/repository/cache -v -count=1 2>&1
if ($LASTEXITCODE -eq 0) {
    Write-Host "OK Cache 测试通过" -ForegroundColor Green
}
else {
    Write-Host "ERROR Cache 测试失败" -ForegroundColor Red
}
Write-Host ""

# 总结
Write-Host "==========================================" -ForegroundColor Cyan
Write-Host "测试完成" -ForegroundColor Cyan
Write-Host "==========================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "提示:" -ForegroundColor Yellow
Write-Host "- 要测试 Kafka 功能，请确保 Kafka 已启动"
Write-Host "- 要进行压测，请安装 wrk 工具"
Write-Host "- 查看详细测试指南: docs/TESTING_GUIDE.md"
Write-Host ""
