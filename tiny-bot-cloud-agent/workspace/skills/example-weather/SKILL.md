---
name: weather
description: 查询某城市的实时天气。返回 1 行简短结果。
parameters:
  city:
    type: string
    description: 城市名，例如"杭州"
    required: true
script: scripts/run.sh
---

# Weather skill

读 stdin 收到的 JSON，取 `city` 字段，返回 1 行字符串。
真实实现会调外部天气 API；这里用 mock 返回固定数据。
