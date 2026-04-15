#!/bin/bash

# 快速测试脚本 - 测试单个功能

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

show_menu() {
    echo ""
    echo "=========================================="
    echo "  功能测试菜单"
    echo "=========================================="
    echo "1. 测试接口限流"
    echo "2. 测试 Kafka 重试和死信队列"
    echo "3. 测试幂等性"
    echo "4. 测试热榜缓存一致性"
    echo "5. 登录接口压测"
    echo "6. 查看 Redis 键"
    echo "7. 查看 Kafka 消费者组状态"
    echo "8. 运行所有测试"
    echo "0. 退出"
    echo "=========================================="
    echo -n "请选择 [0-8]: "
}

test_ratelimit() {
    echo -e "${BLUE}测试接口限流...${NC}"
    echo ""
    
    echo "1. 测试全局限流（发送 10 个并发请求）"
    for i in {1..10}; do
        curl -s -X GET http://localhost:8081/rank/daily > /dev/null &
    done
    wait
    echo -e "${GREEN}✓ 请求已发送${NC}"
    echo ""
    
    echo "2. 测试登录限流（连续 6 次登录）"
    for i in {1..6}; do
        response=$(curl -s -w "\n%{http_code}" -X POST http://localhost:8081/user/login \
            -H "Content-Type: application/json" \
            -d '{"email":"test@example.com","password":"Test123!@#"}')
        status_code=$(echo "$response" | tail -n1)
        
        if [ "$status_code" == "429" ]; then
            echo -e "  第 $i 次: ${RED}429 (被限流)${NC}"
        else
            echo "  第 $i 次: $status_code"
        fi
        sleep 0.5
    done
    echo ""
    echo -e "${GREEN}✓ 限流测试完成${NC}"
}

test_kafka() {
    echo -e "${BLUE}测试 Kafka 消息...${NC}"
    echo ""
    
    echo "发送测试消息到 user.registered..."
    echo '{"user_id":888,"email":"kafka-test@example.com"}' | \
        MSYS_NO_PATHCONV=1 docker exec -i user-center-kafka-1 /opt/kafka/bin/kafka-console-producer.sh \
            --bootstrap-server localhost:9092 \
            --topic user.registered 2>/dev/null
    
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ 消息已发送${NC}"
        echo "请查看 worker 日志观察处理情况"
    else
        echo -e "${RED}✗ 消息发送失败${NC}"
        echo "请确保 Kafka 容器已启动: docker ps | grep kafka"
    fi
}

test_idempotent() {
    echo -e "${BLUE}测试幂等性...${NC}"
    echo ""
    
    echo "发送 3 条相同的消息..."
    for i in {1..3}; do
        echo '{"user_id":777,"email":"idempotent-test@example.com"}' | \
            docker exec -i user-center-kafka-1 /opt/kafka/bin/kafka-console-producer.sh \
                --bootstrap-server localhost:9092 \
                --topic user.registered 2>/dev/null
        
        if [ $? -eq 0 ]; then
            echo "  第 $i 条消息已发送"
        else
            echo -e "  第 $i 条消息发送失败" -ForegroundColor Red
        fi
        sleep 1
    done
    
    echo ""
    echo -e "${GREEN}✓ 消息已发送${NC}"
    echo "请查看 worker 日志，应该只处理 1 次"
}

test_rank_consistency() {
    echo -e "${BLUE}测试热榜缓存一致性...${NC}"
    echo ""
    
    if [ -f "test/concurrent_rank_update.go" ]; then
        go run test/concurrent_rank_update.go
    else
        echo -e "${RED}✗ 测试文件不存在: test/concurrent_rank_update.go${NC}"
    fi
}

test_benchmark() {
    echo -e "${BLUE}登录接口压测...${NC}"
    echo ""
    
    if command -v wrk &> /dev/null; then
        echo "使用 wrk 进行压测..."
        wrk -t4 -c100 -d10s --latency \
            -s script/benchmark_login.lua \
            http://localhost:8081/user/login
    else
        echo -e "${YELLOW}⚠ wrk 未安装，使用 curl 进行简单测试${NC}"
        echo "发送 100 个请求..."
        
        start_time=$(date +%s)
        for i in {1..100}; do
            curl -s -X POST http://localhost:8081/user/login \
                -H "Content-Type: application/json" \
                -d '{"email":"test@example.com","password":"Test123!@#"}' > /dev/null &
            
            if [ $((i % 10)) -eq 0 ]; then
                wait
            fi
        done
        wait
        end_time=$(date +%s)
        
        duration=$((end_time - start_time))
        qps=$((100 / duration))
        
        echo ""
        echo "总耗时: ${duration}s"
        echo "QPS: ~${qps}"
    fi
}

view_redis_keys() {
    echo -e "${BLUE}查看 Redis 键...${NC}"
    echo ""
    
    echo "1. 限流键:"
    redis-cli --scan --pattern '*limiter*' 2>/dev/null | head -n 5
    count=$(redis-cli --scan --pattern '*limiter*' 2>/dev/null | wc -l)
    echo "   总数: $count"
    echo ""
    
    echo "2. 幂等键:"
    redis-cli --scan --pattern 'idempotent:*' 2>/dev/null | head -n 5
    count=$(redis-cli --scan --pattern 'idempotent:*' 2>/dev/null | wc -l)
    echo "   总数: $count"
    echo ""
    
    echo "3. 热榜键:"
    redis-cli --scan --pattern 'rank:*' 2>/dev/null | head -n 5
    count=$(redis-cli --scan --pattern 'rank:*' 2>/dev/null | wc -l)
    echo "   总数: $count"
    echo ""
    
    echo "4. 版本键:"
    redis-cli --scan --pattern '*version*' 2>/dev/null | head -n 5
    count=$(redis-cli --scan --pattern '*version*' 2>/dev/null | wc -l)
    echo "   总数: $count"
}

view_kafka_status() {
    echo -e "${BLUE}查看 Kafka 状态...${NC}"
    echo ""
    
    echo "1. Topic 列表:"
    MSYS_NO_PATHCONV=1 docker exec user-center-kafka-1 /opt/kafka/bin/kafka-topics.sh \
        --list --bootstrap-server localhost:9092 2>/dev/null
    echo ""
    
    echo "2. 消费者组状态:"
    MSYS_NO_PATHCONV=1 docker exec user-center-kafka-1 /opt/kafka/bin/kafka-consumer-groups.sh \
        --bootstrap-server localhost:9092 \
        --describe \
        --group user-center-worker 2>/dev/null
}

run_all_tests() {
    echo -e "${BLUE}运行所有测试...${NC}"
    bash test/run_all_tests.sh
}

# 主循环
while true; do
    show_menu
    read -r choice
    
    case $choice in
        1) test_ratelimit ;;
        2) test_kafka ;;
        3) test_idempotent ;;
        4) test_rank_consistency ;;
        5) test_benchmark ;;
        6) view_redis_keys ;;
        7) view_kafka_status ;;
        8) run_all_tests ;;
        0) echo "退出"; exit 0 ;;
        *) echo -e "${RED}无效选择${NC}" ;;
    esac
    
    echo ""
    read -p "按 Enter 继续..."
done
