package model

import "testing"

// ===== ParseModelName =====

func TestParseModelName_Plain(t *testing.T) {
	base, thinking, search, tools := ParseModelName("GLM-4.7")
	if base != "GLM-4.7" {
		t.Errorf("base = %q, want %q", base, "GLM-4.7")
	}
	if thinking || search || tools {
		t.Errorf("flags = (%v, %v, %v), want all false", thinking, search, tools)
	}
}

func TestParseModelName_Thinking(t *testing.T) {
	base, thinking, search, tools := ParseModelName("GLM-4.7-thinking")
	if base != "GLM-4.7" {
		t.Errorf("base = %q", base)
	}
	if !thinking {
		t.Error("thinking should be true")
	}
	if search || tools {
		t.Error("search and tools should be false")
	}
}

func TestParseModelName_Search(t *testing.T) {
	base, thinking, search, tools := ParseModelName("GLM-4.7-search")
	if base != "GLM-4.7" {
		t.Errorf("base = %q", base)
	}
	if !search {
		t.Error("search should be true")
	}
	if thinking || tools {
		t.Error("thinking and tools should be false")
	}
}

func TestParseModelName_Tools(t *testing.T) {
	base, thinking, search, tools := ParseModelName("GLM-4.7-tools")
	if base != "GLM-4.7" {
		t.Errorf("base = %q", base)
	}
	if !tools {
		t.Error("tools should be true")
	}
	if thinking || search {
		t.Error("thinking and search should be false")
	}
}

func TestParseModelName_ThinkingSearch(t *testing.T) {
	base, thinking, search, tools := ParseModelName("GLM-4.7-thinking-search")
	if base != "GLM-4.7" {
		t.Errorf("base = %q", base)
	}
	if !thinking || !search {
		t.Error("thinking and search should both be true")
	}
	if tools {
		t.Error("tools should be false")
	}
}

func TestParseModelName_ToolsThinking(t *testing.T) {
	base, thinking, search, tools := ParseModelName("GLM-4.7-tools-thinking")
	if base != "GLM-4.7" {
		t.Errorf("base = %q", base)
	}
	if !tools || !thinking {
		t.Error("tools and thinking should both be true")
	}
	if search {
		t.Error("search should be false")
	}
}

func TestParseModelName_ToolsSearch(t *testing.T) {
	base, thinking, search, tools := ParseModelName("GLM-4.7-tools-search")
	if base != "GLM-4.7" {
		t.Errorf("base = %q", base)
	}
	if !tools || !search {
		t.Error("tools and search should both be true")
	}
	if thinking {
		t.Error("thinking should be false")
	}
}

func TestParseModelName_AllTags(t *testing.T) {
	base, thinking, search, tools := ParseModelName("GLM-4.7-tools-thinking-search")
	if base != "GLM-4.7" {
		t.Errorf("base = %q", base)
	}
	if !thinking || !search || !tools {
		t.Errorf("all flags should be true, got (%v, %v, %v)", thinking, search, tools)
	}
}

func TestParseModelName_ReverseOrder(t *testing.T) {
	base, thinking, search, tools := ParseModelName("GLM-4.7-search-thinking-tools")
	if base != "GLM-4.7" {
		t.Errorf("base = %q", base)
	}
	if !thinking || !search || !tools {
		t.Errorf("all flags should be true, got (%v, %v, %v)", thinking, search, tools)
	}
}

// ===== IsToolsModel =====

func TestIsToolsModel_True(t *testing.T) {
	tests := []string{
		"GLM-4.7-tools",
		"GLM-4.7-tools-thinking",
		"GLM-4.7-tools-search",
		"GLM-4.7-thinking-tools",
		"GLM-4.5-tools",
	}
	for _, m := range tests {
		if !IsToolsModel(m) {
			t.Errorf("IsToolsModel(%q) = false, want true", m)
		}
	}
}

func TestIsToolsModel_False(t *testing.T) {
	tests := []string{
		"GLM-4.7",
		"GLM-4.7-thinking",
		"GLM-4.7-search",
		"GLM-4.7-thinking-search",
	}
	for _, m := range tests {
		if IsToolsModel(m) {
			t.Errorf("IsToolsModel(%q) = true, want false", m)
		}
	}
}

// ===== IsThinkingModel / IsSearchModel 不受 -tools 影响 =====

func TestIsThinkingModel_WithTools(t *testing.T) {
	if !IsThinkingModel("GLM-4.7-tools-thinking") {
		t.Error("IsThinkingModel should be true for GLM-4.7-tools-thinking")
	}
	if IsThinkingModel("GLM-4.7-tools") {
		t.Error("IsThinkingModel should be false for GLM-4.7-tools")
	}
}

func TestIsSearchModel_WithTools(t *testing.T) {
	if !IsSearchModel("GLM-4.7-tools-search") {
		t.Error("IsSearchModel should be true for GLM-4.7-tools-search")
	}
	if IsSearchModel("GLM-4.7-tools") {
		t.Error("IsSearchModel should be false for GLM-4.7-tools")
	}
}

// ===== GetTargetModel with -tools =====

func TestGetTargetModel_WithTools(t *testing.T) {
	target := GetTargetModel("GLM-4.7-tools")
	if target != "glm-4.7" {
		t.Errorf("GetTargetModel(GLM-4.7-tools) = %q, want %q", target, "glm-4.7")
	}
}

func TestGetTargetModel_WithToolsThinking(t *testing.T) {
	target := GetTargetModel("GLM-4.7-tools-thinking")
	if target != "glm-4.7" {
		t.Errorf("GetTargetModel(GLM-4.7-tools-thinking) = %q, want %q", target, "glm-4.7")
	}
}

// ===== ModelList 包含 -tools 变体 =====

func TestModelList_ContainsToolsVariants(t *testing.T) {
	expected := map[string]bool{
		"GLM-4.7-tools":          false,
		"GLM-4.7-tools-thinking": false,
	}

	for _, m := range ModelList {
		if _, ok := expected[m]; ok {
			expected[m] = true
		}
	}

	for name, found := range expected {
		if !found {
			t.Errorf("ModelList missing %q", name)
		}
	}
}

func TestResolveClaudeModel_WithExplicitTools(t *testing.T) {
	resolved, thinking := ResolveClaudeModel("claude-sonnet-4-6", false)
	if resolved != "GLM-4.7" {
		t.Fatalf("resolved = %q, want GLM-4.7", resolved)
	}
	if thinking {
		t.Fatal("thinking should be false")
	}
}

func TestResolveClaudeModel_WithoutTools(t *testing.T) {
	resolved, thinking := ResolveClaudeModel("claude-sonnet-4-6", false)
	if resolved != "GLM-4.7" {
		t.Fatalf("resolved = %q, want GLM-4.7", resolved)
	}
	if thinking {
		t.Fatal("thinking should be false")
	}
}

func TestResolveClaudeModel_OpusStillEnablesThinking(t *testing.T) {
	resolved, thinking := ResolveClaudeModel("claude-opus-4-6", false)
	if resolved != "GLM-4.7-thinking" {
		t.Fatalf("resolved = %q, want GLM-4.7-thinking", resolved)
	}
	if !thinking {
		t.Fatal("thinking should be true for opus models")
	}
}
