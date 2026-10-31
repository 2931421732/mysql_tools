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

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type Client struct {
	BaseURL    string
	APIKey     string
	Model      string
	HTTPClient *http.Client
}

func NewClient(baseURL, apiKey, model string) *Client {
	if model == "" {
		model = "deepseek-chat"
	}
	return &Client{BaseURL: strings.TrimRight(baseURL, "/"), APIKey: apiKey, Model: model, HTTPClient: &http.Client{Timeout: 90 * time.Second}}
}

func (c *Client) endpoint() string {
	base := strings.TrimRight(c.BaseURL, "/")
	if strings.HasSuffix(base, "/chat/completions") {
		return base
	}
	return base + "/chat/completions"
}

type chatRequest struct {
	Model          string    `json:"model"`
	Messages       []Message `json:"messages"`
	Temperature    float64   `json:"temperature,omitempty"`
	MaxTokens      int       `json:"max_tokens,omitempty"`
	ResponseFormat any       `json:"response_format,omitempty"`
	Stream         bool      `json:"stream"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage Usage `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// Chat sends one completion request. It retries without JSON mode when the
// endpoint rejects response_format, which keeps proxies compatible.
func (c *Client) Chat(ctx context.Context, messages []Message, temperature float64, maxTokens int) (string, Usage, error) {
	if strings.TrimSpace(c.APIKey) == "" {
		return "", Usage{}, fmt.Errorf("DeepSeek API Key 未配置")
	}
	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{Timeout: 90 * time.Second}
	}
	reqBody := chatRequest{
		Model: c.Model, Messages: messages, Temperature: temperature, MaxTokens: maxTokens,
		ResponseFormat: map[string]string{"type": "json_object"},
	}
	content, usage, status, err := c.do(ctx, reqBody)
	if err != nil && status == http.StatusBadRequest {
		reqBody.ResponseFormat = nil
		content, usage, _, err = c.do(ctx, reqBody)
	}
	return content, usage, err
}

func (c *Client) do(ctx context.Context, reqBody chatRequest) (string, Usage, int, error) {
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", Usage{}, 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint(), bytes.NewReader(payload))
	if err != nil {
		return "", Usage{}, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", Usage{}, 0, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return "", Usage{}, res.StatusCode, err
	}
	var parsed chatResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", Usage{}, res.StatusCode, fmt.Errorf("DeepSeek 返回了无法解析的响应（HTTP %d）", res.StatusCode)
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		msg := strings.TrimSpace(string(raw))
		if parsed.Error != nil && parsed.Error.Message != "" {
			msg = parsed.Error.Message
		}
		if len(msg) > 400 {
			msg = msg[:400]
		}
		return "", Usage{}, res.StatusCode, fmt.Errorf("DeepSeek API %d: %s", res.StatusCode, msg)
	}
	if len(parsed.Choices) == 0 {
		return "", parsed.Usage, res.StatusCode, fmt.Errorf("DeepSeek 没有返回候选结果")
	}
	return parsed.Choices[0].Message.Content, parsed.Usage, res.StatusCode, nil
}
