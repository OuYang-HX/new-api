// Copyright (C) 2023-2026 QuantumNous
// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// TokenCache provides a thread-safe in-memory cache for tokens keyed by config name.
type TokenCache struct {
	mu    sync.RWMutex
	cache map[string]string
}

var globalTokenCache = &TokenCache{cache: make(map[string]string)}

// Get returns the cached token value for the given name.
func (tc *TokenCache) Get(name string) (string, bool) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	v, ok := tc.cache[name]
	return v, ok
}

// Set stores a token value under the given name.
func (tc *TokenCache) Set(name string, value string) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.cache[name] = value
}

// Delete removes the token for the given name.
func (tc *TokenCache) Delete(name string) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	delete(tc.cache, name)
}

// GetTokenByName returns the cached token for a config name.
func GetTokenByName(name string) (string, bool) {
	return globalTokenCache.Get(name)
}

// ResolveTokenVariables replaces all ${token:name} patterns in value with the
// corresponding cached token. Unknown names are left as-is.
func ResolveTokenVariables(value string, _ int) string {
	var b strings.Builder
	rest := value
	for {
		start := strings.Index(rest, "${token:")
		if start == -1 {
			b.WriteString(rest)
			break
		}
		b.WriteString(rest[:start])
		end := strings.Index(rest[start:], "}")
		if end == -1 {
			b.WriteString(rest[start:])
			break
		}
		name := rest[start+8 : start+end]
		if tok, ok := globalTokenCache.Get(name); ok {
			b.WriteString(tok)
		} else {
			b.WriteString(rest[start : start+end+1])
		}
		rest = rest[start+end+1:]
	}
	return b.String()
}

// StartTokenRefreshScheduler starts the background token refresh loop.
// It performs an initial load and then checks every 30 seconds.
func StartTokenRefreshScheduler() {
	go func() {
		refreshAllTokens()
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			refreshAllTokens()
		}
	}()
}

// refreshAllTokens loads all enabled configs and refreshes expired ones.
func refreshAllTokens() {
	configs, err := model.GetAllEnabledTokenConfigs()
	if err != nil {
		common.SysError(fmt.Sprintf("failed to load token configs: %v", err))
		return
	}
	now := common.GetTimestamp()
	for _, cfg := range configs {
		if cfg.CurrentToken != "" && cfg.TokenExpiresAt > now {
			globalTokenCache.Set(cfg.Name, cfg.CurrentToken)
			continue
		}
		token, expiresAt, err := fetchToken(cfg)
		if err != nil {
			common.SysError(fmt.Sprintf("failed to refresh token %s: %v", cfg.Name, err))
			continue
		}
		cfg.CurrentToken = token
		cfg.TokenExpiresAt = expiresAt
		if err := model.DB.Save(cfg).Error; err != nil {
			common.SysError(fmt.Sprintf("failed to save token config %s: %v", cfg.Name, err))
		}
		globalTokenCache.Set(cfg.Name, token)
	}
}

// ManualRefreshToken forces a refresh of a specific token config by ID.
func ManualRefreshToken(id int) (*model.TokenConfig, error) {
	cfg, err := model.GetTokenConfigById(id)
	if err != nil {
		return nil, err
	}
	token, expiresAt, err := fetchToken(cfg)
	if err != nil {
		return nil, err
	}
	cfg.CurrentToken = token
	cfg.TokenExpiresAt = expiresAt
	if err := model.DB.Save(cfg).Error; err != nil {
		return nil, fmt.Errorf("failed to save token config: %w", err)
	}
	globalTokenCache.Set(cfg.Name, token)
	return cfg, nil
}

// fetchToken performs the login request described by cfg and extracts the token.
func fetchToken(cfg *model.TokenConfig) (token string, expiresAt int64, err error) {
	// Build request body with variable substitution
	body := cfg.LoginBody
	body = strings.ReplaceAll(body, "{username}", cfg.Username)
	body = strings.ReplaceAll(body, "{password}", cfg.Password)

	method := cfg.LoginMethod
	if method == "" {
		method = http.MethodPost
	}

	var bodyReader io.Reader
	if body != "" {
		bodyReader = bytes.NewBufferString(body)
	}

	req, err := http.NewRequest(method, cfg.LoginURL, bodyReader)
	if err != nil {
		return "", 0, fmt.Errorf("build request: %w", err)
	}

	// Parse and set headers
	if cfg.LoginHeaders != "" {
		headers, parseErr := parseLoginHeaders(cfg.LoginHeaders)
		if parseErr != nil {
			return "", 0, fmt.Errorf("parse headers: %w", parseErr)
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
	}
	// Default content type when body is present and none was set
	if body != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: common.TLSInsecureSkipVerify,
			},
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", 0, fmt.Errorf("login returned status %d: %s", resp.StatusCode, truncateString(string(respBody), 256))
	}

	// Parse response JSON
	var data map[string]interface{}
	if err := common.Unmarshal(respBody, &data); err != nil {
		return "", 0, fmt.Errorf("parse response json: %w", err)
	}

	// Extract token via JSONPath
	token, err = extractTokenByJSONPath(data, cfg.TokenJSONPath)
	if err != nil {
		return "", 0, fmt.Errorf("extract token: %w", err)
	}

	refreshInterval := int64(cfg.RefreshInterval)
	if refreshInterval <= 0 {
		refreshInterval = 3600
	}
	expiresAt = common.GetTimestamp() + refreshInterval

	return token, expiresAt, nil
}

// parseLoginHeaders parses a JSON string into a header map.
func parseLoginHeaders(raw string) (map[string]string, error) {
	var headers map[string]string
	if err := common.Unmarshal([]byte(raw), &headers); err != nil {
		return nil, err
	}
	return headers, nil
}

// extractTokenByJSONPath performs simple dot-notation JSONPath extraction.
// Supports patterns like $.result.token, $.data.access_token.
// Strips the optional $. prefix, splits by '.', and walks the map.
func extractTokenByJSONPath(data map[string]interface{}, path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("empty json path")
	}

	p := path
	// Strip $. prefix
	if strings.HasPrefix(p, "$.") {
		p = p[2:]
	} else if strings.HasPrefix(p, "$") {
		p = p[1:]
		if p == "" {
			return "", fmt.Errorf("invalid json path: %s", path)
		}
	}

	parts := strings.Split(p, ".")
	if len(parts) == 0 {
		return "", fmt.Errorf("invalid json path: %s", path)
	}

	var current interface{} = data
	for i, part := range parts {
		if part == "" {
			return "", fmt.Errorf("empty segment at position %d in path: %s", i, path)
		}
		m, ok := current.(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("segment %q is not an object at position %d in path: %s", part, i, path)
		}
		current, ok = m[part]
		if !ok {
			return "", fmt.Errorf("key %q not found at position %d in path: %s", part, i, path)
		}
	}

	str, ok := current.(string)
	if !ok {
		return fmt.Sprintf("%v", current), nil
	}
	return str, nil
}

// truncateString truncates s to at most maxLen characters.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// DeleteTokenFromCache removes a token from the in-memory cache by config name.
func DeleteTokenFromCache(name string) {
	globalTokenCache.Delete(name)
}
