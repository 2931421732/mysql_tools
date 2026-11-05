package server

import (
	"fmt"
	"strings"

	"navmini/internal/ai"
	"navmini/internal/config"
)

func normalizeAIProvider(p string) string {
	switch strings.ToLower(strings.TrimSpace(p)) {
	case "claude-code", "claudecode", "claude":
		return "claude-code"
	case "codex":
		return "codex"
	default:
		return "deepseek"
	}
}

func protocolForAIProvider(p string) string {
	switch normalizeAIProvider(p) {
	case "claude-code":
		return "anthropic-messages"
	case "codex":
		return "openai-responses"
	default:
		return "openai-chat"
	}
}

func (s *Server) resolveAIProfile() (ai.ProviderProfile, error) {
	cfg := s.store.GetAI()
	if cfg.Source == "local" {
		return ai.DetectLocal(cfg.Provider, cfg.LocalPath)
	}
	key := s.store.AIKey()
	if key == "" {
		return ai.ProviderProfile{}, fmt.Errorf("尚未配置 API Key")
	}
	base := strings.TrimSpace(cfg.BaseURL)
	if base == "" {
		base = "https://api.deepseek.com"
	}
	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = "deepseek-chat"
	}
	return ai.ProviderProfile{
		Provider: normalizeAIProvider(cfg.Provider), Protocol: protocolForAIProvider(cfg.Provider),
		BaseURL: base, Model: model, APIKey: key, AuthStyle: "bearer",
	}, nil
}

func (s *Server) aiStatePayload() map[string]any {
	cfg := s.store.GetAI()
	out := map[string]any{
		"enabled": cfg.Enabled, "provider": normalizeAIProvider(cfg.Provider),
		"source": cfg.Source, "localPath": cfg.LocalPath,
		"baseURL": cfg.BaseURL, "model": cfg.Model,
		"permission": cfg.Permission, "databases": cfg.Databases,
		"autoExplain": cfg.AutoExplain, "hasKey": cfg.HasKey,
		"models": []string{"deepseek-chat", "deepseek-reasoner"},
	}
	profile, err := s.resolveAIProfile()
	if err != nil {
		out["localError"] = err.Error()
		if cfg.Source == "local" {
			out["hasKey"] = false
		}
		return out
	}
	out["hasKey"] = profile.APIKey != ""
	out["baseURL"] = profile.BaseURL
	out["model"] = profile.Model
	out["protocol"] = profile.Protocol
	out["configPath"] = profile.ConfigPath
	return out
}

func profileFromRequest(cfg config.AIConfig, base, model, key, provider, source, localPath string) (ai.ProviderProfile, error) {
	source = strings.ToLower(strings.TrimSpace(source))
	if source == "" {
		source = cfg.Source
	}
	if strings.TrimSpace(provider) == "" {
		provider = cfg.Provider
	}
	if source == "local" {
		if strings.TrimSpace(localPath) == "" {
			localPath = cfg.LocalPath
		}
		return ai.DetectLocal(provider, localPath)
	}
	if strings.TrimSpace(base) == "" {
		base = cfg.BaseURL
	}
	if strings.TrimSpace(model) == "" {
		model = cfg.Model
	}
	if strings.TrimSpace(key) == "" {
		key = ""
	}
	base = strings.TrimSpace(base)
	if base == "" {
		base = "https://api.deepseek.com"
	}
	model = strings.TrimSpace(model)
	if model == "" {
		model = "deepseek-chat"
	}
	style := "bearer"
	if normalizeAIProvider(provider) == "claude-code" && strings.Contains(strings.ToLower(base), "anthropic.com") {
		style = "x-api-key"
	}
	return ai.ProviderProfile{
		Provider: normalizeAIProvider(provider), Protocol: protocolForAIProvider(provider),
		BaseURL: base, Model: model, APIKey: strings.TrimSpace(key), AuthStyle: style,
	}, nil
}
