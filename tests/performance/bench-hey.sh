#!/bin/bash
#
# wr-proxy 压力测试脚本 (基于 hey)
#
# 用法: ./bench-hey.sh [模式]
#   模式: all(默认) | quick | medium | heavy | auto-model | fallback
#
# 依赖: hey (已安装到 $HOME/go/bin)
# 目标: http://localhost:5051 (wr-proxy)
#
# 注意: 压测前确保 wr-proxy 正在运行
#       cd webrouter/wr-proxy && make run
#

set -euo pipefail

WR_PROXY="http://localhost:5051"
API_KEY="${WR_PROXY_KEY:-sk-wr-1848e7b46537f135af70e8d402f57adeae6d2630}"
MODEL="gpt-4o"

HEADER_AUTH="Authorization: Bearer $API_KEY"
HEADER_JSON="Content-Type: application/json"

BODY_SMALL='{"model":"'"$MODEL"'","messages":[{"role":"user","content":"hi"}]}'
BODY_MEDIUM='{"model":"'"$MODEL"'","messages":[{"role":"user","content":"Write a short explanation of how HTTP load balancing works, in about 200 words."}]}'
BODY_LARGE='{"model":"'"$MODEL"'","messages":[{"role":"user","content":"Write a detailed technical document comparing REST, GraphQL, and gRPC architectures. Cover protocol design, performance characteristics, tooling ecosystem, and use-case suitability. Aim for 1000+ words."}]}'

REPORT_DIR="/root/.openclaw/workspace/webrouter/tests/performance/results"
mkdir -p "$REPORT_DIR"

run_test() {
    local name="$1"
    local url="$2"
    local body="$3"
    local n="$4"
    local c="$5"
    local output="$REPORT_DIR/${name}_$(date +%Y%m%d_%H%M%S).txt"

    echo ""
    echo "========================================"
    echo "  测试: $name"
    echo "  请求数: $n | 并发: $c | 目标: $url"
    echo "========================================"

    export PATH="$PATH:$HOME/go/bin"
    hey -n "$n" -c "$c" -m POST \
        -H "$HEADER_AUTH" \
        -H "$HEADER_JSON" \
        -d "$body" \
        -o csv "$url" 2>/dev/null | tee "$output" || true

    echo ""
    echo "--- 结果已保存到: $output ---"
}

run_summary_test() {
    local name="$1"
    local url="$2"
    local body="$3"
    local n="$4"
    local c="$5"
    local output="$REPORT_DIR/${name}_summary_$(date +%Y%m%d_%H%M%S).txt"

    echo ""
    echo "========================================"
    echo "  测试: $name (摘要)"
    echo "  请求数: $n | 并发: $c | 目标: $url"
    echo "========================================"

    export PATH="$PATH:$HOME/go/bin"
    hey -n "$n" -c "$c" -m POST \
        -H "$HEADER_AUTH" \
        -H "$HEADER_JSON" \
        -d "$body" "$url" 2>&1 | tee "$output" || true

    echo ""
    echo "--- 摘要已保存到: $output ---"
}

MODE="${1:-all}"

case "$MODE" in
    quick)
        # 快速冒烟: 50 请求, 10 并发
        run_summary_test "quick_small" \
            "$WR_PROXY/v1/chat/completions" "$BODY_SMALL" 50 10
        ;;

    medium)
        # 中等负载: 200 请求, 25 并发, 中等 body
        run_summary_test "medium" \
            "$WR_PROXY/v1/chat/completions" "$BODY_MEDIUM" 200 25
        ;;

    heavy)
        # 重负载: 500 请求, 50 并发, 大 body
        run_summary_test "heavy" \
            "$WR_PROXY/v1/chat/completions" "$BODY_LARGE" 500 50
        ;;

    auto-model)
        # 自动模型选择: 混合小/中请求测试 auto 模式
        run_summary_test "auto_small" \
            "$WR_PROXY/v1/chat/completions" \
            '{"model":"auto","messages":[{"role":"user","content":"hi"}]}' \
            100 20
        echo ""
        run_summary_test "auto_medium" \
            "$WR_PROXY/v1/chat/completions" \
            '{"model":"auto","messages":[{"role":"user","content":"Write a short explanation of how HTTP load balancing works."}]}' \
            100 20
        ;;

    fallback)
        # 降级测试: 高并发小请求, 观察降级行为
        run_summary_test "fallback_high_concurrency" \
            "$WR_PROXY/v1/chat/completions" "$BODY_SMALL" 300 50
        ;;

    all)
        echo "===== wr-proxy 全面压力测试 ====="
        echo "开始时间: $(date)"

        run_summary_test "quick_small" \
            "$WR_PROXY/v1/chat/completions" "$BODY_SMALL" 50 10

        run_summary_test "medium" \
            "$WR_PROXY/v1/chat/completions" "$BODY_MEDIUM" 200 25

        run_summary_test "auto_small" \
            "$WR_PROXY/v1/chat/completions" \
            '{"model":"auto","messages":[{"role":"user","content":"hello"}]}' \
            100 20

        run_summary_test "auto_medium" \
            "$WR_PROXY/v1/chat/completions" \
            '{"model":"smart","messages":[{"role":"user","content":"Explain microservices architecture briefly."}]}' \
            100 20

        echo ""
        echo "===== 全部测试完成 ====="
        echo "结束时间: $(date)"
        echo "结果目录: $REPORT_DIR"
        ;;

    *)
        echo "用法: $0 [all|quick|medium|heavy|auto-model|fallback]"
        exit 1
        ;;
esac
