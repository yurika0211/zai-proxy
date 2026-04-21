package tools

import (
	"encoding/json"
	"math"
	"testing"

	"zai-proxy/internal/model"
)

func TestResolveEffectiveTools_PrefersClientDefinitions(t *testing.T) {
	resolved := ResolveEffectiveTools("GLM-4.7-tools", []model.Tool{{
		Type: "function",
		Function: model.ToolFunction{
			Name: "calculate",
		},
	}})

	calculateCount := 0
	for _, tool := range resolved.Tools {
		if tool.Function.Name == "calculate" {
			calculateCount++
		}
	}

	if calculateCount != 1 {
		t.Fatalf("expected calculate to appear once, got %d", calculateCount)
	}
	if _, ok := resolved.InjectedBuiltinNames["calculate"]; ok {
		t.Fatal("client-defined tool should not be treated as injected builtin")
	}
	if !resolved.HasInjectedBuiltins() {
		t.Fatal("expected other builtin tools to still be injected")
	}
	if len(resolved.Tools) != len(GetBuiltinTools()) {
		t.Fatalf("expected %d effective tools, got %d", len(GetBuiltinTools()), len(resolved.Tools))
	}
}

func TestExecuteBuiltinToolCalculate(t *testing.T) {
	raw := ExecuteBuiltinTool("calculate", `{"expression":"2+3*4"}`)

	var resp struct {
		OK     bool   `json:"ok"`
		Tool   string `json:"tool"`
		Result struct {
			Value float64 `json:"value"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if !resp.OK {
		t.Fatalf("expected success response, got %s", raw)
	}
	if resp.Tool != "calculate" {
		t.Fatalf("Tool = %q, want calculate", resp.Tool)
	}
	if math.Abs(resp.Result.Value-14) > 1e-9 {
		t.Fatalf("value = %v, want 14", resp.Result.Value)
	}
}

func TestExecuteBuiltinToolDisabled(t *testing.T) {
	raw := ExecuteBuiltinTool("search_web", `{"query":"golang"}`)

	var resp struct {
		OK    bool `json:"ok"`
		Error struct {
			Type string `json:"type"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if resp.OK {
		t.Fatalf("expected disabled tool response, got %s", raw)
	}
	if resp.Error.Type != "tool_disabled" {
		t.Fatalf("error.type = %q, want tool_disabled", resp.Error.Type)
	}
}
