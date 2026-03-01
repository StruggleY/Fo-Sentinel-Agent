package chat

import (
	"Fo-Sentinel-Agent/api/chat/v1"
	"Fo-Sentinel-Agent/internal/ai/agent/plan_execute_replan"
	"context"
	"errors"
)

// AIOps 处理 AI 运维自动化请求，对应路由 POST /api/ai_ops。
// 采用 Plan-Execute-Replan 架构：Planner 拆解任务 → Executor 逐步执行工具调用 → Replanner 评估并调整计划。
// Result 为最终告警分析报告，Detail 为各步骤中间输出
func (c *ControllerV1) AIOps(ctx context.Context, req *v1.AIOpsReq) (res *v1.AIOpsRes, err error) {
	// 固定运维指令：获取活跃告警 → 检索处理方案 → 查询日志 → 汇总生成报告
	query := `
你是一个智能的服务告警运维分析助手，请按以下步骤完成巡检任务：

## 执行步骤

1. 调用工具 query_prometheus_alerts 获取所有活跃告警。
2. 若存在活跃告警，针对每个告警名称分别调用工具 query_internal_docs 检索对应处理方案；若无活跃告警，直接跳到步骤 5。
3. 严格遵循内部文档内容进行分析，不得使用文档以外的知识。
4. 涉及时间参数时，先调用 get_current_time 获取当前毫秒时间戳，再计算所需时间范围后传参；涉及日志查询时必须携带地域和日志主题 ID。
5. 完成所有查询后，按下方格式输出最终报告。

## 输出格式要求

- 使用 Markdown 格式，层级标题用 ##、###
- 数据用列表或表格呈现，禁止大段堆砌文字
- 若无活跃告警，对应章节写"暂无"，不得省略章节

## 报告模板

---
# 告警巡检报告

## 一、活跃告警清单
（无告警时写：当前系统运行正常，无活跃告警）

## 二、告警根因分析
（每条告警单独一个 ### 小节，无告警时写：暂无）

### 告警名称：{alertname}
- **告警描述**：
- **持续时间**：
- **根因研判**：（依据内部文档）

## 三、处理方案执行
（每条告警单独一个 ### 小节，无告警时写：暂无）

### 告警名称：{alertname}
- **处理步骤**：（严格按内部文档）
- **日志核查结果**：
- **执行状态**：已完成 / 待确认 / 无需处理

## 四、结论
- **系统健康状态**：
- **告警数量**：
- **处理完成情况**：
- **后续建议**：
---
`

	resp, detail, err := plan_execute_replan.BuildPlanAgent(ctx, query)
	if err != nil {
		return nil, err
	}
	if resp == "" {
		return nil, errors.New("内部错误")
	}
	res = &v1.AIOpsRes{
		Result: resp,
		Detail: detail,
	}
	return res, nil
}
