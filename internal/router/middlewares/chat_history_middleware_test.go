package middlewares

import (
	"testing"

	"github.com/nanami9426/imgo/internal/models"
)

func TestConsumeConversationOptions(t *testing.T) {
	payload := map[string]interface{}{
		"conversation_id": "123",
		"new_chat":        true,
		"messages":        []interface{}{},
	}

	conversationID, hasConversationID, newChat, err := consumeConversationOptions(payload)
	if err != nil {
		t.Fatalf("consumeConversationOptions error: %v", err)
	}
	if !hasConversationID || conversationID != 123 {
		t.Fatalf("unexpected conversationID: has=%v id=%d", hasConversationID, conversationID)
	}
	if !newChat {
		t.Fatalf("expected newChat=true")
	}
	if _, ok := payload["conversation_id"]; ok {
		t.Fatalf("conversation_id should be removed from payload")
	}
	if _, ok := payload["new_chat"]; ok {
		t.Fatalf("new_chat should be removed from payload")
	}
}

func TestValidateContinueMessages(t *testing.T) {
	valid := []chatMessagePayload{
		{Role: "system", Content: "You are helpful."},
		{Role: "user", Content: "hello"},
	}
	if err := validateContinueMessages(valid); err != nil {
		t.Fatalf("expected valid continue payload, got err=%v", err)
	}

	invalid := []chatMessagePayload{
		{Role: "user", Content: "Q1"},
		{Role: "user", Content: "Q2"},
	}
	if err := validateContinueMessages(invalid); err == nil {
		t.Fatalf("expected invalid continue payload to fail")
	}
}

func TestMergeHistoryAndCurrentMessagesKeepsOrder(t *testing.T) {
	history := []*models.LLMConversationMessage{
		{
			Role:        "assistant",
			Content:     "Hello",
			MessageJSON: `{"role":"assistant","content":"Hello"}`,
		},
	}
	system := &models.LLMConversationMessage{
		Role:        "system",
		Content:     "System prompt",
		MessageJSON: `{"role":"system","content":"System prompt"}`,
	}
	current := []chatMessagePayload{
		{
			Role:    "user",
			Content: "继续聊",
			Raw: map[string]interface{}{
				"role":    "user",
				"content": "继续聊",
			},
		},
	}

	merged, err := mergeHistoryAndCurrentMessages(history, system, current)
	if err != nil {
		t.Fatalf("mergeHistoryAndCurrentMessages error: %v", err)
	}
	if len(merged) != 3 {
		t.Fatalf("expected 3 merged messages, got %d", len(merged))
	}
	if merged[0]["role"] != "system" || merged[1]["role"] != "assistant" || merged[2]["role"] != "user" {
		t.Fatalf("unexpected order after merge: %#v", merged)
	}
}

func TestSplitCurrentMessagesUsesLatestSystem(t *testing.T) {
	current := []chatMessagePayload{
		{
			Role:    "system",
			Content: "old system",
			Raw: map[string]interface{}{
				"role":    "system",
				"content": "old system",
			},
		},
		{
			Role:    "user",
			Content: "hello",
			Raw: map[string]interface{}{
				"role":    "user",
				"content": "hello",
			},
		},
		{
			Role:    "system",
			Content: "new system",
			Raw: map[string]interface{}{
				"role":    "system",
				"content": "new system",
			},
		},
	}

	system, nonSystem := splitCurrentMessages(current)
	if system == nil {
		t.Fatalf("expected system message")
	}
	if system.Content != "new system" {
		t.Fatalf("expected latest system to win, got %q", system.Content)
	}
	if len(nonSystem) != 1 || nonSystem[0].Role != "user" {
		t.Fatalf("expected only user in non-system list, got %#v", nonSystem)
	}
}

func TestExtractAssistantContentAndModelFromJSON(t *testing.T) {
	body := []byte(`{"id":"chatcmpl-1","model":"qwen","choices":[{"message":{"role":"assistant","content":"你好"}}]}`)

	content, model := extractAssistantContentAndModel(body)
	if content != "你好" {
		t.Fatalf("expected assistant content 你好, got %q", content)
	}
	if model != "qwen" {
		t.Fatalf("expected model qwen, got %q", model)
	}
}

func TestExtractAssistantContentAndModelFromSSE(t *testing.T) {
	body := []byte(
		"data: {\"id\":\"chatcmpl-sse\",\"model\":\"qwen\",\"choices\":[{\"delta\":{\"role\":\"assistant\"}}]}\n\n" +
			"data: {\"choices\":[{\"delta\":{\"content\":\"你\"}}]}\n\n" +
			"data: {\"choices\":[{\"delta\":{\"content\":\"好\"}}]}\n\n" +
			"data: [DONE]\n\n",
	)

	content, model := extractAssistantContentAndModel(body)
	if content != "你好" {
		t.Fatalf("expected assistant content 你好, got %q", content)
	}
	if model != "qwen" {
		t.Fatalf("expected model qwen, got %q", model)
	}
}
