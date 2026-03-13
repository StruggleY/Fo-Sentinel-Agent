package observe

import (
	"context"

	e_mcp "github.com/cloudwego/eino-ext/components/tool/mcp"
	"github.com/cloudwego/eino/components/tool"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// GetLogMcpTool 获取腾讯云 CLS 日志查询 MCP 工具列表。
// MCP 接口地址从配置 tools.cls_mcp_url 读取；未配置时返回空列表（跳过日志工具，不影响其他工具）。
//
// 参考文档：
//   - https://cloud.tencent.com/developer/mcp/server/11710
//   - https://www.cloudwego.io/zh/docs/eino/ecosystem_integration/tool/tool_mcp/
func GetLogMcpTool(ctx context.Context) ([]tool.BaseTool, error) {
	mcpURL := g.Cfg().MustGet(ctx, "tools.cls_mcp_url").String()
	if mcpURL == "" {
		g.Log().Warningf(ctx, "[Tools] tools.cls_mcp_url 未配置，跳过腾讯云 CLS 日志查询工具")
		return []tool.BaseTool{}, nil
	}

	cli, err := client.NewSSEMCPClient(mcpURL)
	if err != nil {
		return []tool.BaseTool{}, err
	}
	if err = cli.Start(ctx); err != nil {
		return []tool.BaseTool{}, err
	}
	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "fo-sentinel-agent",
		Version: "1.0.0",
	}
	if _, err = cli.Initialize(ctx, initRequest); err != nil {
		return []tool.BaseTool{}, err
	}
	mcpTools, err := e_mcp.GetTools(ctx, &e_mcp.Config{Cli: cli})
	if err != nil {
		return []tool.BaseTool{}, err
	}
	return mcpTools, nil
}
