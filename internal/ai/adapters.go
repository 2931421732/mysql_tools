package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ChatClient is the common interface used by the assistant for all providers.
type ChatClient interface {
	Chat(ctx context.Context, messages []Message, temperature float64, maxTokens int) (string, Usage, error)
	ListModels(ctx context.Context) ([]string, error)
}

// NewProviderClient returns the protocol adapter for a resolved profile.
func NewProviderClient(p ProviderProfile) ChatClient {
	switch p.Protocol {
	case "anthropic-messages":
		return &AnthropicClient{Profile: p, HTTPClient: &http.Client{Timeout: 90 * time.Second}}
	case "openai-responses":
		return &ResponsesClient{Profile: p, HTTPClient: &http.Client{Timeout: 90 * time.Second}}
	default:
		return NewClient(p.BaseURL, p.APIKey, p.Model)
	}
}

type AnthropicClient struct {
	Profile    ProviderProfile
	HTTPClient *http.Client
}

type anthropicRequest struct {
	Model       string             `json:"model"`
	MaxTokens   int                `json:"max_tokens"`
	Temperature float64            `json:"temperature,omitempty"`
	System      string             `json:"system,omitempty"`
	Messages    []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

func anthropicMessagesURL(base string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	switch {
	case strings.HasSuffix(base, "/v1/messages"):
		return base
	case strings.HasSuffix(base, "/v1"):
		return base + "/messages"
	default:
		return base + "/v1/messages"
	}
}

func (c *AnthropicClient) Chat(ctx context.Context, messages []Message, temperature float64, maxTokens int) (string, Usage, error) {
	// Some Claude models (e.g. Sonnet 5.5 behind LiteLLM) only accept
	// temperature=1 and reject any other value. Temperature is optional in the
	// Anthropic Messages API, so omit it entirely and use the endpoint default.
	_ = temperature
	var system []string
	msgs := make([]anthropicMessage, 0, len(messages))
	for _, m := range messages {
		if m.Role == "system" {
			if strings.TrimSpace(m.Content) != "" {
				system = append(system, m.Content)
			}
			continue
		}
		role := "user"
		if m.Role == "assistant" {
			role = "assistant"
		}
		msgs = append(msgs, anthropicMessage{Role: role, Content: m.Content})
	}
	if maxTokens <= 0 {
		maxTokens = 1800
	}
	reqBody := anthropicRequest{
		Model: c.Profile.Model, MaxTokens: maxTokens,
		System: strings.Join(system, "\n\n"), Messages: msgs,
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", Usage{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicMessagesURL(c.Profile.BaseURL), bytes.NewReader(payload))
	if err != nil {
		return "", Usage{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")
	if c.Profile.AuthStyle == "x-api-key" {
		req.Header.Set("x-api-key", c.Profile.APIKey)
	} else {
		req.Header.Set("Authorization", "Bearer "+c.Profile.APIKey)
	}
	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 90 * time.Second}
	}
	res, err := client.Do(req)
	if err != nil {
		return "", Usage{}, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return "", Usage{}, err
	}
	var parsed anthropicResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", Usage{}, fmt.Errorf("Anthropic 返回无法解析（HTTP %d）", res.StatusCode)
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		msg := strings.TrimSpace(string(raw))
		if parsed.Error != nil && parsed.Error.Message != "" {
			msg = parsed.Error.Message
		}
		if len(msg) > 500 {
			msg = msg[:500]
		}
		return "", Usage{}, fmt.Errorf("Anthropic API %d: %s", res.StatusCode, msg)
	}
	var text strings.Builder
	for _, part := range parsed.Content {
		if part.Type == "text" {
			text.WriteString(part.Text)
		}
	}
	usage := Usage{PromptTokens: parsed.Usage.InputTokens, CompletionTokens: parsed.Usage.OutputTokens}
	usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	return text.String(), usage, nil
}

func (c *AnthropicClient) ListModels(ctx context.Context) ([]string, error) {
	if strings.TrimSpace(c.Profile.Model) == "" {
		return nil, fmt.Errorf("Claude Code 配置中没有模型")
	}
	return []string{c.Profile.Model}, nil
}

type ResponsesClient struct {
	Profile    ProviderProfile
	HTTPClient *http.Client
}

type responsesRequest struct {
	Model           string             `json:"model"`
	Input           []responsesMessage `json:"input"`
	Instructions    string             `json:"instructions,omitempty"`
	MaxOutputTokens int                `json:"max_output_tokens,omitempty"`
	Temperature     float64            `json:"temperature,omitempty"`
}

type responsesMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

func responsesURL(base string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	switch {
	case strings.HasSuffix(base, "/responses"):
		return base
	case strings.HasSuffix(base, "/v1"):
		return base + "/responses"
	default:
		return base + "/responses"
	}
}

func (c *ResponsesClient) Chat(ctx context.Context, messages []Message, temperature float64, maxTokens int) (string, Usage, error) {
	var instructions []string
	input := make([]responsesMessage, 0, len(messages))
	for _, m := range messages {
		if m.Role == "system" {
			if strings.TrimSpace(m.Content) != "" {
				instructions = append(instructions, m.Content)
			}
			continue
		}
		role, contentType := "user", "input_text"
		if m.Role == "assistant" {
			role, contentType = "assistant", "output_text"
		}
		input = append(input, responsesMessage{Role: role, Content: []map[string]string{{"type": contentType, "text": m.Content}}})
	}
	reqBody := responsesRequest{
		Model: c.Profile.Model, Input: input,
		Instructions: strings.Join(instructions, "\n\n"),
		Temperature:  temperature,
	}
	if maxTokens > 0 {
		reqBody.MaxOutputTokens = maxTokens
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", Usage{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, responsesURL(c.Profile.BaseURL), bytes.NewReader(payload))
	if err != nil {
		return "", Usage{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Profile.APIKey)
	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 90 * time.Second}
	}
	res, err := client.Do(req)
	if err != nil {
		return "", Usage{}, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return "", Usage{}, err
	}
	var parsed struct {
		OutputText string `json:"output_text"`
		Output     []struct {
			Type    string `json:"type"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			InputTokens      int `json:"input_tokens"`
			OutputTokens     int `json:"output_tokens"`
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", Usage{}, fmt.Errorf("Responses 返回无法解析（HTTP %d）", res.StatusCode)
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		msg := strings.TrimSpace(string(raw))
		if parsed.Error != nil && parsed.Error.Message != "" {
			msg = parsed.Error.Message
		}
		if len(msg) > 500 {
			msg = msg[:500]
		}
		return "", Usage{}, fmt.Errorf("Responses API %d: %s", res.StatusCode, msg)
	}
	text := parsed.OutputText
	if text == "" {
		for _, item := range parsed.Output {
			for _, part := range item.Content {
				if part.Type == "output_text" || part.Type == "text" {
					text += part.Text
				}
			}
		}
	}
	if text == "" && len(parsed.Choices) > 0 {
		text = parsed.Choices[0].Message.Content
	}
	prompt, completion := parsed.Usage.InputTokens, parsed.Usage.OutputTokens
	if prompt == 0 {
		prompt = parsed.Usage.PromptTokens
	}
	if completion == 0 {
		completion = parsed.Usage.CompletionTokens
	}
	return text, Usage{PromptTokens: prompt, CompletionTokens: completion, TotalTokens: prompt + completion}, nil
}

func (c *ResponsesClient) ListModels(ctx context.Context) ([]string, error) {
	base := strings.TrimRight(strings.TrimSpace(c.Profile.BaseURL), "/")
	base = strings.TrimSuffix(base, "/responses")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Profile.APIKey)
	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	res, err := client.Do(req)
	if err != nil {
		if c.Profile.Model != "" {
			return []string{c.Profile.Model}, nil
		}
		return nil, err
	}
	defer res.Body.Close()
	var parsed struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 2<<20)).Decode(&parsed); err != nil || len(parsed.Data) == 0 {
		if c.Profile.Model != "" {
			return []string{c.Profile.Model}, nil
		}
		return nil, fmt.Errorf("Codex /models 没有返回模型")
	}
	out := make([]string, 0, len(parsed.Data))
	for _, m := range parsed.Data {
		if m.ID != "" {
			out = append(out, m.ID)
		}
	}
	return out, nil
}
