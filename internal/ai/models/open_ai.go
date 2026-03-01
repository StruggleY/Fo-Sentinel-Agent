// Package models 封装项目中所有 LLM 的初始化逻辑。
//
// 底层使用 Eino 的 openai 组件，该组件实现了 OpenAI Chat Completions API 协议。
// DeepSeek 和豆包等模型均提供了兼容 OpenAI 协议的接入地址（BaseURL），
// 因此可以直接复用同一个 openai.NewChatModel，只需切换 BaseURL + APIKey + Model 即可对接不同厂商。
package models

import (
	"context"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/gogf/gf/v2/frame/g"
)

// OpenAIForDeepSeekV31Think 初始化深度思考版 LLM（对应配置 ds_think_chat_model）。
// 深度思考模式下，模型在回答前会进行更长的内部推理链（Chain of Thought），
// 适合需要多步逻辑推导的复杂任务，但首 Token 延迟较高，不适合高频工具调用场景。
// 对应配置：base_url=火山引擎, model=deepseek-v3-1-terminus
func OpenAIForDeepSeekV31Think(ctx context.Context) (cm model.ToolCallingChatModel, err error) {
	// 从 GoFrame 配置文件（config.yaml）中读取模型参数，避免硬编码敏感信息
	model, err := g.Cfg().Get(ctx, "ds_think_chat_model.model")
	if err != nil {
		return nil, err
	}
	api_key, err := g.Cfg().Get(ctx, "ds_think_chat_model.api_key")
	if err != nil {
		return nil, err
	}
	base_url, err := g.Cfg().Get(ctx, "ds_think_chat_model.base_url")
	if err != nil {
		return nil, err
	}
	// openai.ChatModelConfig 是 Eino openai 组件的连接配置。
	// BaseURL 指向兼容 OpenAI 协议的第三方服务（火山引擎 ARK），
	// Eino 会将请求发往 BaseURL/chat/completions 端点。
	config := &openai.ChatModelConfig{
		Model:   model.String(),
		APIKey:  api_key.String(),
		BaseURL: base_url.String(),
	}
	// openai.NewChatModel 返回实现了 model.ToolCallingChatModel 接口的实例，
	// 该接口同时支持普通对话（Generate）和 Function Calling（工具调用）两种能力。
	cm, err = openai.NewChatModel(ctx, config)
	if err != nil {
		return nil, err
	}
	return cm, nil
}

// OpenAIForDeepSeekV3Quick 初始化快速响应版 LLM（对应配置 ds_quick_chat_model）。
// 无深度思考，延迟低，适合 ReAct 循环中的多步工具调用场景（每步都需要快速决策调用哪个工具）。
// 对应配置：base_url=火山引擎, model=deepseek-v3-1-terminus
func OpenAIForDeepSeekV3Quick(ctx context.Context) (cm model.ToolCallingChatModel, err error) {
	model, err := g.Cfg().Get(ctx, "ds_quick_chat_model.model")
	if err != nil {
		return nil, err
	}
	api_key, err := g.Cfg().Get(ctx, "ds_quick_chat_model.api_key")
	if err != nil {
		return nil, err
	}
	base_url, err := g.Cfg().Get(ctx, "ds_quick_chat_model.base_url")
	if err != nil {
		return nil, err
	}
	config := &openai.ChatModelConfig{
		Model:   model.String(),
		APIKey:  api_key.String(),
		BaseURL: base_url.String(),
	}
	cm, err = openai.NewChatModel(ctx, config)
	if err != nil {
		return nil, err
	}
	return cm, nil
}
