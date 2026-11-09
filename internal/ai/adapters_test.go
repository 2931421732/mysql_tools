package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestAnthropicChat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/anthropic/v1/messages" || r.Header.Get("Authorization") != "Bearer tok" {
			http.Error(w, "bad path/auth", http.StatusBadRequest)
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["model"] != "m" || body["system"] != "sys" {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		if _, ok := body["temperature"]; ok {
			http.Error(w, "temperature must be omitted", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"hello"}],"usage":{"input_tokens":3,"output_tokens":2}}`))
	}))
	defer srv.Close()
	c := NewProviderClient(ProviderProfile{Protocol: "anthropic-messages", BaseURL: srv.URL + "/anthropic", Model: "m", APIKey: "tok", AuthStyle: "bearer"})
	text, usage, err := c.Chat(context.Background(), []Message{{Role: "system", Content: "sys"}, {Role: "user", Content: "hi"}}, 0, 32)
	if err != nil {
		t.Fatal(err)
	}
	if text != "hello" || usage.TotalTokens != 5 {
		t.Fatalf("text=%q usage=%#v", text, usage)
	}
}

func TestResponsesChat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" || r.Header.Get("Authorization") != "Bearer tok" {
			http.Error(w, "bad path/auth", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"output":[{"type":"message","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":4,"output_tokens":1}}`))
	}))
	defer srv.Close()
	c := NewProviderClient(ProviderProfile{Protocol: "openai-responses", BaseURL: srv.URL, Model: "m", APIKey: "tok"})
	text, _, err := c.Chat(context.Background(), []Message{{Role: "user", Content: "hi"}}, 0, 32)
	if err != nil || text != "ok" {
		t.Fatalf("text=%q err=%v", text, err)
	}
}

func TestDetectClaudeLocal(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "settings.json")
	_ = os.WriteFile(p, []byte(`{"env":{"ANTHROPIC_BASE_URL":"https://x/anthropic","ANTHROPIC_AUTH_TOKEN":"tok","ANTHROPIC_MODEL":"cm"}}`), 0o600)
	got, err := DetectLocal("claude-code", p)
	if err != nil {
		t.Fatal(err)
	}
	if got.BaseURL != "https://x/anthropic" || got.Model != "cm" || got.APIKey != "tok" || got.Protocol != "anthropic-messages" {
		t.Fatalf("got %#v", got)
	}
}

func TestDetectCodexLocal(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "config.toml"), []byte(`model = "gpt-x"
model_provider = "p"
[model_providers.p]
base_url = "https://x"
wire_api = "responses"
experimental_bearer_token = "tok"
`), 0o600)
	got, err := DetectLocal("codex", dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.BaseURL != "https://x" || got.Model != "gpt-x" || got.APIKey != "tok" || got.Protocol != "openai-responses" {
		t.Fatalf("got %#v", got)
	}
}
