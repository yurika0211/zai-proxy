package tools

import "zai-proxy/internal/model"

// EffectiveTools 描述最终注入给模型的工具集合，以及其中哪些是代理自动补入的内置工具。
type EffectiveTools struct {
	Tools                []model.Tool
	InjectedBuiltinNames map[string]struct{}
}

func (e EffectiveTools) HasInjectedBuiltins() bool {
	return len(e.InjectedBuiltinNames) > 0
}

// ResolveEffectiveTools 统一计算请求侧和响应侧都应使用的有效工具集合。
// 客户端显式定义的同名工具优先，内置工具只补缺失项。
func ResolveEffectiveTools(modelName string, clientTools []model.Tool) EffectiveTools {
	effective := append([]model.Tool(nil), clientTools...)
	clientToolNames := make(map[string]struct{}, len(clientTools))
	for _, t := range clientTools {
		clientToolNames[t.Function.Name] = struct{}{}
	}

	injectedBuiltinNames := make(map[string]struct{})
	if model.IsToolsModel(modelName) {
		for _, builtin := range GetBuiltinTools() {
			if _, exists := clientToolNames[builtin.Function.Name]; exists {
				continue
			}
			effective = append(effective, builtin)
			injectedBuiltinNames[builtin.Function.Name] = struct{}{}
		}
	}

	return EffectiveTools{
		Tools:                effective,
		InjectedBuiltinNames: injectedBuiltinNames,
	}
}
