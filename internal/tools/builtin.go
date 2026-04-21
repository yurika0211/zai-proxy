package tools

import (
	"fmt"
	"strings"

	"zai-proxy/internal/config"
	"zai-proxy/internal/model"
)

// GetBuiltinTools 返回当前会被 `-tools` 自动注入的内置工具定义。
// 这里只保留代理能够真正执行的 builtin tools，避免向模型暴露注定失败的工具。
func GetBuiltinTools() []model.Tool {
	tools := []model.Tool{
		{
			Type: "function",
			Function: model.ToolFunction{
				Name:        "get_current_time",
				Description: "获取当前时间，支持不同时区和格式",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"timezone": map[string]interface{}{
							"type":        "string",
							"description": "时区名称（如 Asia/Shanghai, America/New_York）",
						},
						"format": map[string]interface{}{
							"type":        "string",
							"description": "时间格式（如 2006-01-02 15:04:05）",
						},
					},
					"required": []string{},
				},
			},
		},
		{
			Type: "function",
			Function: model.ToolFunction{
				Name:        "calculate",
				Description: "执行数学计算，支持基本运算和高级数学函数",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"expression": map[string]interface{}{
							"type":        "string",
							"description": "数学表达式（如 2+3*4, sqrt(16), sin(pi/2)）",
						},
					},
					"required": []string{"expression"},
				},
			},
		},
	}

	execSettings := config.CurrentExecCommandSettings()
	if execSettings.Enabled {
		tools = append(tools, model.Tool{
			Type: "function",
			Function: model.ToolFunction{
				Name:        "exec_command",
				Description: fmt.Sprintf("在受控白名单内执行终端命令。适合查看目录、读取文件、运行测试或启动开发服务器。仅允许这些命令前缀：%s。不支持管道、重定向、&&、|| 等 shell 语法。", strings.Join(execSettings.Allowlist, ", ")),
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"command": map[string]interface{}{
							"type":        "string",
							"description": "要执行的命令。可以写成简单命令行（如 npm run dev, go test ./...），但不能包含 shell 运算符。",
						},
						"args": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"type": "string",
							},
							"description": "可选。推荐显式传参；如果提供 args，command 应只包含可执行文件名。",
						},
						"workdir": map[string]interface{}{
							"type":        "string",
							"description": "可选。执行目录，必须位于代理允许的工作目录内。相对路径按配置工作目录解析。",
						},
						"timeout_sec": map[string]interface{}{
							"type":        "integer",
							"description": "可选。前台执行超时秒数；默认使用代理配置值。",
						},
						"run_in_background": map[string]interface{}{
							"type":        "boolean",
							"description": "可选。为 true 时后台启动长时间运行的进程，并返回 pid 与日志文件路径。",
						},
						"description": map[string]interface{}{
							"type":        "string",
							"description": "可选。对这次执行目的的简短说明。",
						},
					},
					"required": []string{"command"},
				},
			},
		})
	}

	return tools
}
