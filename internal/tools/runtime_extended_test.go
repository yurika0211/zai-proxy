package tools

import (
	"encoding/json"
	"testing"
)

func TestExecuteCurrentTimeTool_DefaultFormat(t *testing.T) {
	args := map[string]interface{}{
		"timezone": "",
		"format":   "",
	}

	result := executeCurrentTimeTool(args)
	if result == "" {
		t.Error("expected non-empty result")
	}

	var response toolExecutionResponse
	if err := json.Unmarshal([]byte(result), &response); err != nil {
		t.Errorf("expected valid JSON response, got error: %v", err)
	}

	if !response.OK {
		t.Error("expected OK=true")
	}
	if response.Tool != "get_current_time" {
		t.Errorf("expected tool 'get_current_time', got %q", response.Tool)
	}
}

func TestExecuteCurrentTimeTool_CustomFormat(t *testing.T) {
	args := map[string]interface{}{
		"timezone": "",
		"format":   "2006-01-02",
	}

	result := executeCurrentTimeTool(args)
	if result == "" {
		t.Error("expected non-empty result")
	}

	var response toolExecutionResponse
	if err := json.Unmarshal([]byte(result), &response); err != nil {
		t.Errorf("expected valid JSON response, got error: %v", err)
	}

	if !response.OK {
		t.Error("expected OK=true")
	}
}

func TestExecuteCurrentTimeTool_ValidTimezone(t *testing.T) {
	args := map[string]interface{}{
		"timezone": "America/New_York",
		"format":   "2006-01-02 15:04:05",
	}

	result := executeCurrentTimeTool(args)
	if result == "" {
		t.Error("expected non-empty result")
	}

	var response toolExecutionResponse
	if err := json.Unmarshal([]byte(result), &response); err != nil {
		t.Errorf("expected valid JSON response, got error: %v", err)
	}

	if !response.OK {
		t.Error("expected OK=true for valid timezone")
	}
}

func TestExecuteCurrentTimeTool_InvalidTimezone(t *testing.T) {
	args := map[string]interface{}{
		"timezone": "Invalid/Timezone",
		"format":   "2006-01-02 15:04:05",
	}

	result := executeCurrentTimeTool(args)
	if result == "" {
		t.Error("expected non-empty result")
	}

	var response toolExecutionResponse
	if err := json.Unmarshal([]byte(result), &response); err != nil {
		t.Errorf("expected valid JSON response, got error: %v", err)
	}

	if response.OK {
		t.Error("expected OK=false for invalid timezone")
	}
	if response.Error == nil {
		t.Error("expected error for invalid timezone")
	}
	if response.Error.Type != "invalid_timezone" {
		t.Errorf("expected error type 'invalid_timezone', got %q", response.Error.Type)
	}
}

func TestExecuteCurrentTimeTool_UTCTimezone(t *testing.T) {
	args := map[string]interface{}{
		"timezone": "UTC",
		"format":   "2006-01-02 15:04:05",
	}

	result := executeCurrentTimeTool(args)
	if result == "" {
		t.Error("expected non-empty result")
	}

	var response toolExecutionResponse
	if err := json.Unmarshal([]byte(result), &response); err != nil {
		t.Errorf("expected valid JSON response, got error: %v", err)
	}

	if !response.OK {
		t.Error("expected OK=true for UTC timezone")
	}
}

func TestExecuteCurrentTimeTool_NoArgs(t *testing.T) {
	args := map[string]interface{}{}

	result := executeCurrentTimeTool(args)
	if result == "" {
		t.Error("expected non-empty result")
	}

	var response toolExecutionResponse
	if err := json.Unmarshal([]byte(result), &response); err != nil {
		t.Errorf("expected valid JSON response, got error: %v", err)
	}

	if !response.OK {
		t.Error("expected OK=true with no args")
	}
}

func TestExecuteCurrentTimeTool_ResultHasTimestamp(t *testing.T) {
	args := map[string]interface{}{
		"timezone": "UTC",
		"format":   "2006-01-02 15:04:05",
	}

	result := executeCurrentTimeTool(args)

	var response toolExecutionResponse
	json.Unmarshal([]byte(result), &response)

	if response.Result == nil {
		t.Error("expected result field")
	}

	resultMap, ok := response.Result.(map[string]interface{})
	if !ok {
		t.Error("expected result to be a map")
	}

	if _, ok := resultMap["time"]; !ok {
		t.Error("expected time in result")
	}
	if _, ok := resultMap["unix"]; !ok {
		t.Error("expected unix in result")
	}
	if _, ok := resultMap["timezone"]; !ok {
		t.Error("expected timezone in result")
	}
}

func TestEvalMathFunction_Sqrt(t *testing.T) {
	result, err := evalMathFunction("sqrt", []float64{4})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result != 2.0 {
		t.Errorf("expected 2, got %v", result)
	}
}

func TestEvalMathFunction_Sin(t *testing.T) {
	result, err := evalMathFunction("sin", []float64{0})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result != 0.0 {
		t.Errorf("expected 0, got %v", result)
	}
}

func TestEvalMathFunction_Abs(t *testing.T) {
	result, err := evalMathFunction("abs", []float64{-5})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result != 5.0 {
		t.Errorf("expected 5, got %v", result)
	}
}

func TestEvalMathFunction_Floor(t *testing.T) {
	result, err := evalMathFunction("floor", []float64{3.7})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result != 3.0 {
		t.Errorf("expected 3, got %v", result)
	}
}

func TestEvalMathFunction_Ceil(t *testing.T) {
	result, err := evalMathFunction("ceil", []float64{3.2})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result != 4.0 {
		t.Errorf("expected 4, got %v", result)
	}
}

func TestEvalMathFunction_Round(t *testing.T) {
	result, err := evalMathFunction("round", []float64{3.5})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result != 4.0 {
		t.Errorf("expected 4, got %v", result)
	}
}

func TestEvalMathFunction_UnknownFunction(t *testing.T) {
	_, err := evalMathFunction("unknown", []float64{1})
	if err == nil {
		t.Error("expected error for unknown function")
	}
}

func TestEvalMathFunction_WrongArity(t *testing.T) {
	_, err := evalMathFunction("sqrt", []float64{1, 2})
	if err == nil {
		t.Error("expected error for wrong arity")
	}
}

func TestStartBackgroundExecCommand_Simple(t *testing.T) {
	// This function is hard to test without mocking exec.Command
	// Just verify it doesn't panic with valid input
	// Skipping for now as it requires complex setup
	t.Skip("requires exec.Command mocking")
}

func TestStartBackgroundExecCommand_WithArgs(t *testing.T) {
	t.Skip("requires exec.Command mocking")
}

func TestStartBackgroundExecCommand_NoArgs(t *testing.T) {
	t.Skip("requires exec.Command mocking")
}
