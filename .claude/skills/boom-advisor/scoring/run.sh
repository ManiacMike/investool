#!/bin/bash
# 股票池五维评分：采集 + 计算 + 报告
set -e
cd "$(dirname "$0")"
curl -sf http://localhost:4869/api/boom/data > /dev/null || {
  echo "investool 服务未启动：go run main.go webserver -c config.toml"; exit 1; }
python3 collect.py
python3 compute.py
