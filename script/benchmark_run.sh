#!/bin/bash

# 登录接口压测脚本

BASE_URL=${BASE_URL:-"http://localhost:8081"}
THREADS=${THREADS:-12}
CONNECTIONS=${CONNECTIONS:-400}
DURATION=${DURATION:-30s}

echo "=========================================="
echo "登录接口压测"
echo "=========================================="
echo "目标地址: $BASE_URL"
echo "线程数: $THREADS"
echo "连接数: $CONNECTIONS"
echo "持续时间: $DURATION"
echo "=========================================="
echo ""

# 检查 wrk 是否安装
if ! command -v wrk &> /dev/null; then
    echo "错误: wrk 未安装"
    echo "请安装 wrk: https://github.com/wg/wrk"
    exit 1
fi

# 运行压测
echo "开始压测..."
echo ""

wrk -t${THREADS} -c${CONNECTIONS} -d${DURATION} --latency \
    -s benchmark_login.lua \
    ${BASE_URL}/user/login

echo ""
echo "压测完成！"
