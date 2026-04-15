#!/bin/bash

# Kafka Topic 创建脚本

KAFKA_BROKER=${KAFKA_BROKER:-"localhost:9092"}

echo "正在创建 Kafka Topics..."
echo "Broker: $KAFKA_BROKER"
echo ""

# 业务 Topics
TOPICS=(
    "user.registered"
    "user.activity"
)

# 死信队列 Topics
DLQ_TOPICS=(
    "user.registered.dlq"
    "user.activity.dlq"
)

# 创建业务 Topics
for topic in "${TOPICS[@]}"; do
    echo "创建 Topic: $topic"
    kafka-topics.sh --create \
        --topic "$topic" \
        --bootstrap-server "$KAFKA_BROKER" \
        --partitions 3 \
        --replication-factor 1 \
        --if-not-exists
    
    if [ $? -eq 0 ]; then
        echo "✓ $topic 创建成功"
    else
        echo "✗ $topic 创建失败"
    fi
    echo ""
done

# 创建死信队列 Topics
for topic in "${DLQ_TOPICS[@]}"; do
    echo "创建死信队列: $topic"
    kafka-topics.sh --create \
        --topic "$topic" \
        --bootstrap-server "$KAFKA_BROKER" \
        --partitions 1 \
        --replication-factor 1 \
        --if-not-exists
    
    if [ $? -eq 0 ]; then
        echo "✓ $topic 创建成功"
    else
        echo "✗ $topic 创建失败"
    fi
    echo ""
done

echo "所有 Topics 创建完成！"
echo ""
echo "查看已创建的 Topics:"
kafka-topics.sh --list --bootstrap-server "$KAFKA_BROKER"
