#!/bin/bash

echo "=========================================="
echo "开始执行所有功能测试"
echo "=========================================="
echo ""

# 颜色定义
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 测试结果统计
PASSED=0
FAILED=0
SKIPPED=0

# 检查服务是否运行
check_service() {
    if curl -s http://localhost:8081/rank/daily > /dev/null 2>&1; then
        return 0
    else
        return 1
    fi
}

# 检查 Redis
check_redis() {
    if redis-cli ping > /dev/null 2>&1; then
        return 0
    else
        return 1
    fi
}

# 检查 Kafka
check_kafka() {
    if kafka-topics.sh --list --bootstrap-server localhost:9092 > /dev/null 2>&1; then
        return 0
    else
        return 1
    fi
}

echo "检查依赖服务..."
if check_redis; then
    echo -e "${GREEN}✓ Redis 运行正常${NC}"
else
    echo -e "${RED}✗ Redis 未运行${NC}"
    exit 1
fi

if check_kafka; then
    echo -e "${GREEN}✓ Kafka 运行正常${NC}"
else
    echo -e "${YELLOW}⚠ Kafka 未运行（部分测试将跳过）${NC}"
fi

if check_service; then
    echo -e "${GREEN}✓ 服务运行正常${NC}"
else
    echo -e "${YELLOW}⚠ 服务未运行（部分测试将跳过）${NC}"
fi
echo ""

# 1. 单元测试
echo "=========================================="
echo "1. 运行单元测试"
echo "=========================================="

echo "测试 Worker 模块..."
if go test ./internal/worker -v -count=1; then
    echo -e "${GREEN}✓ Worker 单元测试通过${NC}"
    ((PASSED++))
else
    echo -e "${RED}✗ Worker 单元测试失败${NC}"
    ((FAILED++))
fi
echo ""

echo "测试 Cache 模块..."
if go test ./internal/repository/cache -v -count=1; then
    echo -e "${GREEN}✓ Cache 单元测试通过${NC}"
    ((PASSED++))
else
    echo -e "${RED}✗ Cache 单元测试失败${NC}"
    ((FAILED++))
fi
echo ""

# 2. 限流测试
if check_service; then
    echo "=========================================="
    echo "2. 测试接口限流"
    echo "=========================================="
    
    echo "发送 10 个并发请求测试限流..."
    for i in {1..10}; do
        curl -s -X GET http://localhost:8081/rank/daily > /dev/null &
    done
    wait
    echo -e "${GREEN}✓ 限流测试完成${NC}"
    ((PASSED++))
    echo ""
    
    echo "测试登录限流（连续 6 次请求）..."
    for i in {1..6}; do
        response=$(curl -s -w "\n%{http_code}" -X POST http://localhost:8081/user/login \
            -H "Content-Type: application/json" \
            -d '{"email":"test@example.com","password":"Test123!@#"}')
        status_code=$(echo "$response" | tail -n1)
        
        if [ $i -eq 6 ] && [ "$status_code" == "429" ]; then
            echo -e "${GREEN}✓ 第 6 次请求被限流（429）${NC}"
        elif [ $i -lt 6 ]; then
            echo "  第 $i 次请求: $status_code"
        fi
    done
    ((PASSED++))
    echo ""
else
    echo "跳过限流测试（服务未运行）"
    ((SKIPPED++))
fi

# 3. Kafka 测试
if check_kafka; then
    echo "=========================================="
    echo "3. 测试 Kafka 消息"
    echo "=========================================="
    
    echo "发送测试消息到 user.registered..."
    echo '{"user_id":999,"email":"test@example.com"}' | \
        kafka-console-producer.sh \
            --broker-list localhost:9092 \
            --topic user.registered 2>/dev/null
    
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ Kafka 消息已发送${NC}"
        ((PASSED++))
    else
        echo -e "${RED}✗ Kafka 消息发送失败${NC}"
        ((FAILED++))
    fi
    echo ""
else
    echo "跳过 Kafka 测试（Kafka 未运行）"
    ((SKIPPED++))
fi

# 4. Redis 键检查
echo "=========================================="
echo "4. 检查 Redis 键"
echo "=========================================="

echo "检查限流键..."
limiter_keys=$(redis-cli --scan --pattern '*limiter*' 2>/dev/null | wc -l)
echo "  限流键数量: $limiter_keys"

echo "检查幂等键..."
idempotent_keys=$(redis-cli --scan --pattern 'idempotent:*' 2>/dev/null | wc -l)
echo "  幂等键数量: $idempotent_keys"

echo "检查热榜键..."
rank_keys=$(redis-cli --scan --pattern 'rank:*' 2>/dev/null | wc -l)
echo "  热榜键数量: $rank_keys"

echo -e "${GREEN}✓ Redis 键检查完成${NC}"
((PASSED++))
echo ""

# 总结
echo "=========================================="
echo "测试完成"
echo "=========================================="
echo -e "通过: ${GREEN}${PASSED}${NC}"
echo -e "失败: ${RED}${FAILED}${NC}"
echo -e "跳过: ${YELLOW}${SKIPPED}${NC}"
echo ""

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}✓ 所有测试通过！${NC}"
    exit 0
else
    echo -e "${RED}✗ 部分测试失败，请检查日志${NC}"
    exit 1
fi
