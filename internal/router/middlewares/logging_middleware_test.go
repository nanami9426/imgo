package middlewares

import (
	"testing"

	"github.com/nanami9426/imgo/internal/models"
)

func TestExtractTokenInfoJSONUsageComputedTotal(t *testing.T) {
	body := []byte(`{"model":"gpt-4","usage":{"prompt_tokens":10,"completion_tokens":20}}`)
	usage := &models.APIUsage{}

	extractTokenInfo(body, usage)

	if usage.Model != "gpt-4" {
		t.Fatalf("expected model gpt-4, got %q", usage.Model)
	}
	if usage.InputTokens != 10 || usage.OutputTokens != 20 {
		t.Fatalf("expected input/output 10/20, got %d/%d", usage.InputTokens, usage.OutputTokens)
	}
	if usage.TotalTokens != 30 {
		t.Fatalf("expected total_tokens 30, got %d", usage.TotalTokens)
	}
}

func TestExtractTokenInfoJSONTotalOnly(t *testing.T) {
	body := []byte(`{"usage":{"total_tokens":12.7}}`)
	usage := &models.APIUsage{}

	extractTokenInfo(body, usage)

	if usage.TotalTokens != 12 {
		t.Fatalf("expected total_tokens 12, got %d", usage.TotalTokens)
	}
}

func TestExtractTokenInfoSSELastUsageWins(t *testing.T) {
	body := []byte(
		"data: {\"model\":\"gpt-4\",\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":2}}\n" +
			"\n" +
			"data: {\"model\":\"gpt-4\",\"usage\":{\"prompt_tokens\":3,\"completion_tokens\":4,\"total_tokens\":7}}\n" +
			"data: [DONE]\n",
	)
	usage := &models.APIUsage{}

	extractTokenInfo(body, usage)

	if usage.InputTokens != 3 || usage.OutputTokens != 4 || usage.TotalTokens != 7 {
		t.Fatalf("expected 3/4/7, got %d/%d/%d", usage.InputTokens, usage.OutputTokens, usage.TotalTokens)
	}
}

func TestExtractTokenInfoSSENoUsage(t *testing.T) {
	body := []byte("data: [DONE]\n")
	usage := &models.APIUsage{}

	extractTokenInfo(body, usage)

	if usage.TotalTokens != 0 || usage.InputTokens != 0 || usage.OutputTokens != 0 {
		t.Fatalf("expected zero tokens, got %d/%d/%d", usage.InputTokens, usage.OutputTokens, usage.TotalTokens)
	}
}

func TestExtractTokenInfoErrorMessage(t *testing.T) {
	body := []byte(`{"error":{"message":"bad request"}}`)
	usage := &models.APIUsage{}

	extractTokenInfo(body, usage)

	if usage.ErrorMsg != "bad request" {
		t.Fatalf("expected error message, got %q", usage.ErrorMsg)
	}
}
