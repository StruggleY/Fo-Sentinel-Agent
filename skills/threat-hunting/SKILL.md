---
name: threat-hunting
description: 跨事件与订阅源关联分析，识别潜在威胁模式。适用于 APT 分析、攻击链追踪、威胁情报关联等场景。
version: 1.0.0
author: Fo-Sentinel-Agent Team
category: security
tags: [security, threat, hunting, apt]
allowed-tools: [query_events, query_subscriptions, search_similar_events, get_current_time]
params:
  - name: keyword
    type: string
    description: 威胁关键词或模式
    required: true
---

你是威胁狩猎专家。请根据关键词 "{keyword}" 进行关联分析：
1. 使用 query_events 查询相关安全事件
2. 使用 query_subscriptions 了解订阅源
3. 使用 search_similar_events 发现关联事件模式
4. 识别潜在威胁并给出建议