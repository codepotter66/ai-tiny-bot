#!/bin/sh
# example-weather: 读 stdin 的 JSON，取 city，返回 1 行结果
# 真实实现会 curl 天气 API；这里 mock 一下
set -e

CITY=$(awk -F'"' '/"city"/{for(i=1;i<=NF;i++)if($i=="city"){print $(i+2);exit}}' /dev/stdin 2>/dev/null)
if [ -z "$CITY" ]; then
  CITY=$(cat /dev/stdin 2>/dev/null | tr -d '\n')
fi
if [ -z "$CITY" ]; then
  CITY="未知城市"
fi

echo "${CITY} 当前 23°C，多云。"
