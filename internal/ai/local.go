package ai

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ProviderProfile is a resolved model endpoint. It may come from NavMini's
// encrypted settings or directly from Claude Code / Codex local config.
type ProviderProfile struct {
	Provider   string
	Protocol   string // openai-chat | anthropic-messages | openai-responses
	BaseURL    string
	Model      string
	APIKey     string
	AuthStyle  string // bearer | x-api-key
	ConfigPath string
}

func normalizeProvider(p string) string {
	switch strings.ToLower(strings.TrimSpace(p)) {
	case "claude", "claude-code", "claudecode":
		return "claude-code"
	case "codex":
		return "codex"
	default:
		return "deepseek"
	}
}

func protocolForProvider(p string) string {
	switch normalizeProvider(p) {
	case "claude-code":
		return "anthropic-messages"
	case "codex":
		return "openai-responses"
	default:
		return "openai-chat"
	}
}

func expandPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, strings.TrimPrefix(p, "~/"))
		}
	}
	return p
}

func existingFile(paths ...string) string {
	for _, p := range paths {
		p = expandPath(p)
		if p == "" {
			continue
		}
		if st, err := os.Stat(p); err == nil && st.Mode().IsRegular() {
			return p
		}
	}
	return ""
}

func existingDir(paths ...string) string {
	for _, p := range paths {
		p = expandPath(p)
		if p == "" {
			continue
		}
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			return p
		}
	}
	return ""
}

// DetectLocal resolves a provider from local configuration. It never returns
// secrets in errors and only reads files; it does not modify Claude/Codex config.
func DetectLocal(provider, localPath string) (ProviderProfile, error) {
	switch normalizeProvider(provider) {
	case "claude-code":
		return detectClaude(localPath)
	case "codex":
		return detectCodex(localPath)
	default:
		return detectDeepSeek(localPath)
	}
}

func detectClaude(localPath string) (ProviderProfile, error) {
	home, _ := os.UserHomeDir()
	var candidates []string
	lp := expandPath(localPath)
	if lp != "" {
		if st, err := os.Stat(lp); err == nil && st.IsDir() {
			if strings.EqualFold(filepath.Base(lp), ".claude") {
				candidates = append(candidates, filepath.Join(lp, "settings.local.json"), filepath.Join(lp, "settings.json"))
			}
			candidates = append(candidates,
				filepath.Join(lp, ".claude", "settings.local.json"),
				filepath.Join(lp, ".claude", "settings.json"),
				filepath.Join(lp, "settings.local.json"),
				filepath.Join(lp, "settings.json"))
		} else {
			candidates = append(candidates, lp)
		}
	}
	candidates = append(candidates,
		filepath.Join(home, ".claude", "settings.local.json"),
		filepath.Join(home, ".claude", "settings.json"))
	path := existingFile(candidates...)
	if path == "" {
		return ProviderProfile{}, fmt.Errorf("未找到 Claude Code 配置（~/.claude/settings.json）")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return ProviderProfile{}, err
	}
	var doc struct {
		Env map[string]string `json:"env"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return ProviderProfile{}, fmt.Errorf("Claude Code 配置无法解析: %w", err)
	}
	env := doc.Env
	base := strings.TrimSpace(env["ANTHROPIC_BASE_URL"])
	if base == "" {
		base = "https://api.anthropic.com"
	}
	authToken := strings.TrimSpace(env["ANTHROPIC_AUTH_TOKEN"])
	apiKey := strings.TrimSpace(env["ANTHROPIC_API_KEY"])
	key, style := apiKey, "x-api-key"
	if authToken != "" {
		key, style = authToken, "bearer"
	}
	if key == "" {
		return ProviderProfile{}, fmt.Errorf("Claude Code 配置中未找到 ANTHROPIC_AUTH_TOKEN / ANTHROPIC_API_KEY")
	}
	model := firstNonEmpty(env["ANTHROPIC_MODEL"], env["ANTHROPIC_DEFAULT_SONNET_MODEL"], env["ANTHROPIC_DEFAULT_OPUS_MODEL"])
	if model == "" {
		return ProviderProfile{}, fmt.Errorf("Claude Code 配置中未找到 ANTHROPIC_MODEL")
	}
	return ProviderProfile{
		Provider: "claude-code", Protocol: "anthropic-messages", BaseURL: base,
		Model: model, APIKey: key, AuthStyle: style, ConfigPath: path,
	}, nil
}

func detectCodex(localPath string) (ProviderProfile, error) {
	home, _ := os.UserHomeDir()
	var candidates []string
	lp := expandPath(localPath)
	if lp != "" {
		if st, err := os.Stat(lp); err == nil && st.IsDir() {
			if strings.EqualFold(filepath.Base(lp), ".codex") {
				candidates = append(candidates, filepath.Join(lp, "config.toml"))
			}
			candidates = append(candidates,
				filepath.Join(lp, ".codex", "config.toml"),
				filepath.Join(lp, "config.toml"))
		} else {
			candidates = append(candidates, lp)
		}
	}
	if ch := strings.TrimSpace(os.Getenv("CODEX_HOME")); ch != "" {
		candidates = append(candidates, filepath.Join(ch, "config.toml"))
	}
	candidates = append(candidates, filepath.Join(home, ".codex", "config.toml"))
	path := existingFile(candidates...)
	if path == "" {
		return ProviderProfile{}, fmt.Errorf("未找到 Codex 配置（~/.codex/config.toml）")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return ProviderProfile{}, err
	}
	top, providers, err := parseCodexTOML(string(raw))
	if err != nil {
		return ProviderProfile{}, err
	}
	providerID := strings.TrimSpace(top["model_provider"])
	if providerID == "" {
		providerID = "custom"
	}
	cfg := providers[providerID]
	if cfg == nil {
		return ProviderProfile{}, fmt.Errorf("Codex 配置中未找到 model_providers.%s", providerID)
	}
	base := strings.TrimSpace(cfg["base_url"])
	if base == "" {
		return ProviderProfile{}, fmt.Errorf("Codex provider %s 未配置 base_url", providerID)
	}
	key := strings.TrimSpace(cfg["experimental_bearer_token"])
	if key == "" {
		if envName := strings.TrimSpace(cfg["env_key"]); envName != "" {
			key = strings.TrimSpace(os.Getenv(envName))
		}
	}
	authPath := filepath.Join(filepath.Dir(path), "auth.json")
	if key == "" {
		if b, err := os.ReadFile(authPath); err == nil {
			var auth map[string]any
			if json.Unmarshal(b, &auth) == nil {
				if v, ok := auth["OPENAI_API_KEY"].(string); ok {
					key = strings.TrimSpace(v)
				}
				if key == "" {
					if tokens, ok := auth["tokens"].(map[string]any); ok {
						if v, ok := tokens["access_token"].(string); ok {
							key = strings.TrimSpace(v)
						}
					}
				}
			}
		}
	}
	if key == "" {
		return ProviderProfile{}, fmt.Errorf("Codex provider %s 未找到 API Key（config.toml / auth.json）", providerID)
	}
	model := strings.TrimSpace(top["model"])
	if model == "" {
		return ProviderProfile{}, fmt.Errorf("Codex 配置中未找到 model")
	}
	wire := strings.ToLower(strings.TrimSpace(cfg["wire_api"]))
	protocol := "openai-responses"
	if wire == "chat" || wire == "chat_completions" || wire == "chat-completions" {
		protocol = "openai-chat"
	}
	return ProviderProfile{
		Provider: "codex", Protocol: protocol, BaseURL: base,
		Model: model, APIKey: key, AuthStyle: "bearer", ConfigPath: path,
	}, nil
}

func detectDeepSeek(localPath string) (ProviderProfile, error) {
	base := strings.TrimSpace(os.Getenv("DEEPSEEK_BASE_URL"))
	model := strings.TrimSpace(os.Getenv("DEEPSEEK_MODEL"))
	key := strings.TrimSpace(os.Getenv("DEEPSEEK_API_KEY"))
	configPath := ""
	lp := expandPath(localPath)
	if lp != "" {
		if st, err := os.Stat(lp); err == nil && st.IsDir() {
			lp = filepath.Join(lp, ".env")
		}
		if st, err := os.Stat(lp); err == nil && st.Mode().IsRegular() {
			configPath = lp
			env, err := readDotEnv(lp)
			if err == nil {
				if base == "" {
					base = env["DEEPSEEK_BASE_URL"]
				}
				if model == "" {
					model = env["DEEPSEEK_MODEL"]
				}
				if key == "" {
					key = env["DEEPSEEK_API_KEY"]
				}
			}
		}
	}
	if base == "" {
		base = "https://api.deepseek.com"
	}
	if model == "" {
		model = "deepseek-chat"
	}
	if key == "" {
		return ProviderProfile{}, fmt.Errorf("未找到 DEEPSEEK_API_KEY（环境变量或项目 .env）")
	}
	return ProviderProfile{
		Provider: "deepseek", Protocol: "openai-chat", BaseURL: base,
		Model: model, APIKey: key, AuthStyle: "bearer", ConfigPath: configPath,
	}, nil
}

func readDotEnv(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := map[string]string{}
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(strings.TrimPrefix(k, "export "))
		v = strings.TrimSpace(v)
		v = strings.Trim(v, `"'`)
		out[k] = v
	}
	return out, s.Err()
}

// parseCodexTOML intentionally implements only the subset Codex writes for
// providers. It handles comments, quoted strings and [model_providers.x] tables.
func parseCodexTOML(raw string) (map[string]string, map[string]map[string]string, error) {
	top := map[string]string{}
	providers := map[string]map[string]string{}
	current := ""
	s := bufio.NewScanner(strings.NewReader(raw))
	for s.Scan() {
		line := strings.TrimSpace(stripTOMLComment(s.Text()))
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			name := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"))
			name = strings.Trim(name, `"'`)
			if strings.HasPrefix(name, "model_providers.") {
				id := strings.Trim(strings.TrimPrefix(name, "model_providers."), `"'`)
				current = "provider:" + id
				if providers[id] == nil {
					providers[id] = map[string]string{}
				}
			} else {
				current = ""
			}
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		v = strings.Trim(v, `"'`)
		if strings.HasPrefix(current, "provider:") {
			providers[strings.TrimPrefix(current, "provider:")][k] = v
		} else if current == "" {
			top[k] = v
		}
	}
	if err := s.Err(); err != nil {
		return nil, nil, err
	}
	return top, providers, nil
}

func stripTOMLComment(line string) string {
	inSingle, inDouble, escaped := false, false, false
	for i, r := range line {
		switch {
		case escaped:
			escaped = false
		case r == '\\' && inDouble:
			escaped = true
		case r == '\'' && !inDouble:
			inSingle = !inSingle
		case r == '"' && !inSingle:
			inDouble = !inDouble
		case r == '#' && !inSingle && !inDouble:
			return line[:i]
		}
	}
	return line
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}
