#!/usr/bin/env bash
# Phase-0 TASK-005 热路径基准采集与对比（插件化改造回归安全网）。
#
# 用法：
#   ./scripts/bench-baseline.sh collect   # 采集基线 -> testdata/bench/baseline.txt
#   ./scripts/bench-baseline.sh compare   # 当前代码 vs 基线，违反阈值时退出码 1
#
# 对比策略（每个基准按多次采样的均值比较）：
#   - allocs/op 严格：增加 > ALLOC_TOLERANCE（默认 0）即失败
#   - ns/op    宽松：劣化 > TIME_TOLERANCE_PCT%（默认 15，吸收 runner 抖动）
# 环境变量：BENCH_COUNT（采样次数，默认 6）、TIME_TOLERANCE_PCT、ALLOC_TOLERANCE
#
# 注意：基线与对比应在同类机器上进行；换机器/换 Go 版本后请重新 collect
# （环境信息记录在 testdata/bench/baseline.env.txt）。

set -euo pipefail
cd "$(dirname "$0")/.."

PATTERN='BenchmarkGatewayForward|BenchmarkGatewayService_ParseSSEUsage|BenchmarkParseClaudeUsageFromResponseBody'
PKGS='./internal/service/'
BASELINE=testdata/bench/baseline.txt
COUNT="${BENCH_COUNT:-6}"
TIME_TOLERANCE_PCT="${TIME_TOLERANCE_PCT:-15}"
ALLOC_TOLERANCE="${ALLOC_TOLERANCE:-0}"

run_bench() {
  go test -tags=unit -run '^$' -bench "$PATTERN" -benchmem -count="$COUNT" "$PKGS"
}

# 从基准输出中提取每个基准的均值：name mean_ns mean_allocs
# ns 与 allocs 独立计数；缺任一指标（如未带 -benchmem）的基准跳过并警告，
# 避免除零产生 inf/nan 静默污染对比结果。
summarize() {
  awk '/^Benchmark/ {
    name=$1; sub(/-[0-9]+$/, "", name)
    for (i=2; i<=NF; i++) {
      if ($(i)=="ns/op")     { ns[name]+=$(i-1); nns[name]++ }
      if ($(i)=="allocs/op") { al[name]+=$(i-1); nal[name]++ }
    }
  }
  END {
    for (k in nns) {
      if (nns[k]>0 && nal[k]>0) printf "%s %.2f %.2f\n", k, ns[k]/nns[k], al[k]/nal[k]
      else print "warn: skip " k " (missing ns/op or allocs/op samples, run with -benchmem)" > "/dev/stderr"
    }
  }' "$1" | sort
}

case "${1:-}" in
  collect)
    mkdir -p "$(dirname "$BASELINE")"
    {
      echo "go: $(go version)"
      echo "cpu: $(grep -m1 'model name' /proc/cpuinfo 2>/dev/null | cut -d: -f2- | xargs || echo unknown)"
      echo "date: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
      echo "count: $COUNT"
    } > "$(dirname "$BASELINE")/baseline.env.txt"
    run_bench | tee "$BASELINE"
    echo "基线已写入 $BASELINE"
    ;;
  compare)
    [ -f "$BASELINE" ] || { echo "缺少基线 $BASELINE，先运行: $0 collect" >&2; exit 2; }
    new="$(mktemp)"
    run_bench | tee "$new"
    echo
    if command -v benchstat >/dev/null 2>&1; then
      benchstat "$BASELINE" "$new" || true
      echo
    fi
    base_sum="$(mktemp)"; new_sum="$(mktemp)"
    summarize "$BASELINE" > "$base_sum"
    summarize "$new" > "$new_sum"
    # 基准名集合对比（审计 F-1）：join 只输出两侧共有的基准，增删/改名若不检查
    # 会被静默丢弃，造成"基准消失但 compare 仍绿"的假阴性。
    added="$(join -v2 "$base_sum" "$new_sum" | awk '{print $1}')"
    missing="$(join -v1 "$base_sum" "$new_sum" | awk '{print $1}')"
    table_status=0
    join "$base_sum" "$new_sum" | awk -v tp="$TIME_TOLERANCE_PCT" -v at="$ALLOC_TOLERANCE" '
      {
        name=$1; bns=$2; bal=$3; nns=$4; nal=$5
        dns = (bns>0) ? (nns-bns)/bns*100 : 0
        dal = nal-bal
        status="ok"
        if (dal > at)  { status="FAIL(allocs/op +" dal ")"; fail=1 }
        else if (dns > tp) { status="FAIL(ns/op +" sprintf("%.1f", dns) "%)"; fail=1 }
        printf "%-60s ns/op %10.0f -> %10.0f (%+6.1f%%)  allocs/op %7.1f -> %7.1f  %s\n", name, bns, nns, dns, bal, nal, status
      }
      END { exit fail ? 1 : 0 }' || table_status=$?
    if [ -n "$added" ]; then
      printf 'warn: 以下基准在基线中不存在，未参与对比（新增基准请重新 collect 基线）：\n%s\n' "$added" >&2
    fi
    if [ -n "$missing" ]; then
      printf 'FAIL: 基线中的以下基准在本次运行缺失（被删除/改名？回归网已失效，须修复或重新 collect）：\n%s\n' "$missing" >&2
      exit 1
    fi
    exit "$table_status"
    ;;
  *)
    echo "用法: $0 {collect|compare}" >&2
    exit 2
    ;;
esac
