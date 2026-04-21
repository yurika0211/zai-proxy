package tools

import "testing"

func TestGetBuiltinTools_Count(t *testing.T) {
	tools := GetBuiltinTools()
	if len(tools) != 3 {
		t.Errorf("len(GetBuiltinTools()) = %d, want 3", len(tools))
	}
}

func TestGetBuiltinTools_AllFunction(t *testing.T) {
	for _, tool := range GetBuiltinTools() {
		if tool.Type != "function" {
			t.Errorf("tool %q Type = %q, want %q", tool.Function.Name, tool.Type, "function")
		}
	}
}

func TestGetBuiltinTools_Names(t *testing.T) {
	expected := map[string]bool{
		"get_current_time": true,
		"calculate":        true,
		"exec_command":     true,
	}

	tools := GetBuiltinTools()
	for _, tool := range tools {
		name := tool.Function.Name
		if !expected[name] {
			t.Errorf("unexpected tool name: %q", name)
		}
		delete(expected, name)
	}

	for name := range expected {
		t.Errorf("missing tool: %q", name)
	}
}

func TestGetBuiltinTools_HaveDescriptions(t *testing.T) {
	for _, tool := range GetBuiltinTools() {
		if tool.Function.Description == "" {
			t.Errorf("tool %q has empty description", tool.Function.Name)
		}
	}
}

func TestGetBuiltinTools_HaveParameters(t *testing.T) {
	for _, tool := range GetBuiltinTools() {
		if tool.Function.Parameters == nil {
			t.Errorf("tool %q has nil parameters", tool.Function.Name)
		}
		params, ok := tool.Function.Parameters.(map[string]interface{})
		if !ok {
			t.Errorf("tool %q parameters is not a map", tool.Function.Name)
			continue
		}
		if params["type"] != "object" {
			t.Errorf("tool %q parameters.type = %v, want %q", tool.Function.Name, params["type"], "object")
		}
		if _, ok := params["properties"]; !ok {
			t.Errorf("tool %q parameters missing 'properties'", tool.Function.Name)
		}
	}
}

func TestGetBuiltinTools_NoDuplicateNames(t *testing.T) {
	seen := make(map[string]bool)
	for _, tool := range GetBuiltinTools() {
		if seen[tool.Function.Name] {
			t.Errorf("duplicate tool name: %q", tool.Function.Name)
		}
		seen[tool.Function.Name] = true
	}
}

func TestGetBuiltinTools_ReturnsNewSlice(t *testing.T) {
	a := GetBuiltinTools()
	b := GetBuiltinTools()
	if &a[0] == &b[0] {
		t.Error("GetBuiltinTools should return a new slice each call")
	}
}
