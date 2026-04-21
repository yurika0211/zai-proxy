package tools

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"math"
	"strings"
	"time"
)

type toolExecutionError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type toolExecutionResponse struct {
	OK     bool                `json:"ok"`
	Tool   string              `json:"tool"`
	Result interface{}         `json:"result,omitempty"`
	Error  *toolExecutionError `json:"error,omitempty"`
}

// ExecuteBuiltinTool 返回可直接作为 tool message content 回传给模型的 JSON 字符串。
func ExecuteBuiltinTool(name string, argumentsJSON string) string {
	args, err := decodeToolArguments(argumentsJSON)
	if err != nil {
		return marshalToolExecutionResponse(toolExecutionResponse{
			OK:   false,
			Tool: name,
			Error: &toolExecutionError{
				Type:    "invalid_arguments",
				Message: err.Error(),
			},
		})
	}

	switch name {
	case "get_current_time":
		return executeCurrentTimeTool(args)
	case "calculate":
		return executeCalculateTool(args)
	case "exec_command":
		return executeExecCommandTool(args)
	case "search_web":
		return marshalDisabledTool(name, "该代理当前未配置独立的网页搜索提供方。需要实时检索时，请继续使用上游模型自带的 -search 能力。")
	case "query_database":
		return marshalDisabledTool(name, "该代理当前未配置数据库连接与查询执行器。")
	case "file_operations":
		return marshalDisabledTool(name, "出于安全原因，该代理默认不开放文件系统写入/读取工具。")
	case "call_external_api":
		return marshalDisabledTool(name, "出于安全原因，该代理默认不开放任意外部 API 调用工具。")
	default:
		return marshalToolExecutionResponse(toolExecutionResponse{
			OK:   false,
			Tool: name,
			Error: &toolExecutionError{
				Type:    "unsupported_tool",
				Message: "unsupported builtin tool",
			},
		})
	}
}

func decodeToolArguments(argumentsJSON string) (map[string]interface{}, error) {
	if strings.TrimSpace(argumentsJSON) == "" {
		return map[string]interface{}{}, nil
	}

	var args map[string]interface{}
	if err := json.Unmarshal([]byte(argumentsJSON), &args); err != nil {
		return nil, fmt.Errorf("arguments must be a JSON object: %w", err)
	}
	return args, nil
}

func executeCurrentTimeTool(args map[string]interface{}) string {
	timezone, _ := args["timezone"].(string)
	format, _ := args["format"].(string)
	if format == "" {
		format = "2006-01-02 15:04:05"
	}

	location := time.Local
	locationName := location.String()
	if timezone != "" {
		loaded, err := time.LoadLocation(timezone)
		if err != nil {
			return marshalToolExecutionResponse(toolExecutionResponse{
				OK:   false,
				Tool: "get_current_time",
				Error: &toolExecutionError{
					Type:    "invalid_timezone",
					Message: err.Error(),
				},
			})
		}
		location = loaded
		locationName = timezone
	}

	now := time.Now().In(location)
	return marshalToolExecutionResponse(toolExecutionResponse{
		OK:   true,
		Tool: "get_current_time",
		Result: map[string]interface{}{
			"time":     now.Format(format),
			"timezone": locationName,
			"unix":     now.Unix(),
			"rfc3339":  now.Format(time.RFC3339),
		},
	})
}

func executeCalculateTool(args map[string]interface{}) string {
	expression, _ := args["expression"].(string)
	if strings.TrimSpace(expression) == "" {
		return marshalToolExecutionResponse(toolExecutionResponse{
			OK:   false,
			Tool: "calculate",
			Error: &toolExecutionError{
				Type:    "missing_expression",
				Message: "expression is required",
			},
		})
	}

	value, err := evaluateMathExpression(expression)
	if err != nil {
		return marshalToolExecutionResponse(toolExecutionResponse{
			OK:   false,
			Tool: "calculate",
			Error: &toolExecutionError{
				Type:    "evaluation_error",
				Message: err.Error(),
			},
		})
	}

	return marshalToolExecutionResponse(toolExecutionResponse{
		OK:   true,
		Tool: "calculate",
		Result: map[string]interface{}{
			"expression": expression,
			"value":      value,
		},
	})
}

func marshalDisabledTool(name, message string) string {
	return marshalToolExecutionResponse(toolExecutionResponse{
		OK:   false,
		Tool: name,
		Error: &toolExecutionError{
			Type:    "tool_disabled",
			Message: message,
		},
	})
}

func marshalToolExecutionResponse(resp toolExecutionResponse) string {
	data, err := json.Marshal(resp)
	if err != nil {
		return fmt.Sprintf(`{"ok":false,"tool":%q,"error":{"type":"marshal_error","message":%q}}`, resp.Tool, err.Error())
	}
	return string(data)
}

func evaluateMathExpression(expression string) (float64, error) {
	expr, err := parser.ParseExpr(expression)
	if err != nil {
		return 0, err
	}
	return evalASTExpr(expr)
}

func evalASTExpr(expr ast.Expr) (float64, error) {
	switch n := expr.(type) {
	case *ast.BasicLit:
		if n.Kind != token.INT && n.Kind != token.FLOAT {
			return 0, fmt.Errorf("unsupported literal: %s", n.Value)
		}
		var value float64
		if _, err := fmt.Sscanf(n.Value, "%f", &value); err != nil {
			return 0, err
		}
		return value, nil
	case *ast.ParenExpr:
		return evalASTExpr(n.X)
	case *ast.UnaryExpr:
		value, err := evalASTExpr(n.X)
		if err != nil {
			return 0, err
		}
		switch n.Op {
		case token.ADD:
			return value, nil
		case token.SUB:
			return -value, nil
		default:
			return 0, fmt.Errorf("unsupported unary operator: %s", n.Op)
		}
	case *ast.BinaryExpr:
		left, err := evalASTExpr(n.X)
		if err != nil {
			return 0, err
		}
		right, err := evalASTExpr(n.Y)
		if err != nil {
			return 0, err
		}
		switch n.Op {
		case token.ADD:
			return left + right, nil
		case token.SUB:
			return left - right, nil
		case token.MUL:
			return left * right, nil
		case token.QUO:
			return left / right, nil
		case token.REM:
			return math.Mod(left, right), nil
		default:
			return 0, fmt.Errorf("unsupported binary operator: %s", n.Op)
		}
	case *ast.CallExpr:
		fn, ok := n.Fun.(*ast.Ident)
		if !ok {
			return 0, fmt.Errorf("unsupported function expression")
		}
		args := make([]float64, 0, len(n.Args))
		for _, arg := range n.Args {
			value, err := evalASTExpr(arg)
			if err != nil {
				return 0, err
			}
			args = append(args, value)
		}
		return evalMathFunction(fn.Name, args)
	case *ast.Ident:
		switch strings.ToLower(n.Name) {
		case "pi":
			return math.Pi, nil
		case "e":
			return math.E, nil
		default:
			return 0, fmt.Errorf("unknown identifier: %s", n.Name)
		}
	default:
		return 0, fmt.Errorf("unsupported expression type: %T", expr)
	}
}

func evalMathFunction(name string, args []float64) (float64, error) {
	switch strings.ToLower(name) {
	case "sqrt":
		return requireArity(name, args, 1, math.Sqrt)
	case "sin":
		return requireArity(name, args, 1, math.Sin)
	case "cos":
		return requireArity(name, args, 1, math.Cos)
	case "tan":
		return requireArity(name, args, 1, math.Tan)
	case "abs":
		return requireArity(name, args, 1, math.Abs)
	case "log":
		return requireArity(name, args, 1, math.Log10)
	case "ln":
		return requireArity(name, args, 1, math.Log)
	case "exp":
		return requireArity(name, args, 1, math.Exp)
	case "floor":
		return requireArity(name, args, 1, math.Floor)
	case "ceil":
		return requireArity(name, args, 1, math.Ceil)
	case "round":
		return requireArity(name, args, 1, math.Round)
	case "pow":
		if len(args) != 2 {
			return 0, fmt.Errorf("%s expects 2 arguments", name)
		}
		return math.Pow(args[0], args[1]), nil
	case "min":
		if len(args) == 0 {
			return 0, fmt.Errorf("%s expects at least 1 argument", name)
		}
		min := args[0]
		for _, value := range args[1:] {
			if value < min {
				min = value
			}
		}
		return min, nil
	case "max":
		if len(args) == 0 {
			return 0, fmt.Errorf("%s expects at least 1 argument", name)
		}
		max := args[0]
		for _, value := range args[1:] {
			if value > max {
				max = value
			}
		}
		return max, nil
	default:
		return 0, fmt.Errorf("unsupported function: %s", name)
	}
}

func requireArity(name string, args []float64, expected int, fn func(float64) float64) (float64, error) {
	if len(args) != expected {
		return 0, fmt.Errorf("%s expects %d arguments", name, expected)
	}
	return fn(args[0]), nil
}
