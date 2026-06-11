// Copyright (C) 2023-2026 QuantumNous
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package custom provides decoupled extensions to the upstream new-api.
// All custom features are registered here, keeping upstream files minimally modified.
package custom

import (
	"net/http"
	"net/url"

	"github.com/QuantumNous/new-api/custom/protocol_adapter"
	"github.com/QuantumNous/new-api/custom/token_config"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// RegisterMigrations appends custom model migrations to the GORM AutoMigrate list.
// It also initializes the DB instance for custom packages.
func RegisterMigrations(database *gorm.DB) {
	token_config.SetDB(database)
	database.AutoMigrate(&token_config.TokenConfig{})
}

// RegisterMigrationsFast initializes the DB instance and returns custom models
// that should be added to the fast migration list. The caller appends them.
func RegisterMigrationsFast(database *gorm.DB) []interface{} {
	token_config.SetDB(database)
	return []interface{}{&token_config.TokenConfig{}}
}

// RegisterRoutes registers custom API routes on the given router group.
func RegisterRoutes(userRoute *gin.RouterGroup) {
	tcRoute := userRoute.Group("/token-config")
	tcRoute.GET("/", token_config.GetTokenConfigs)
	tcRoute.POST("/", token_config.CreateTokenConfig)
	tcRoute.PUT("/:id", token_config.UpdateTokenConfig)
	tcRoute.DELETE("/:id", token_config.DeleteTokenConfig)
	tcRoute.POST("/:id/refresh", token_config.ManualRefreshToken)
}

// StartSchedulers launches custom background schedulers.
func StartSchedulers() {
	go token_config.StartTokenRefreshScheduler()
}

// InitProtocolAdapter initializes the protocol adapter by injecting the relay function.
// This must be called before RegisterRelayRoutes to avoid nil function calls.
// The relayFunc parameter is controller.Relay, passed in from main.go to avoid import cycles.
func InitProtocolAdapter(relayFunc func(c *gin.Context, relayFormat types.RelayFormat)) {
	protocol_adapter.SetRelayFunc(relayFunc)
}

// RegisterRelayRoutes registers custom relay routes on the given router group.
// The router group should have TokenAuth and Distribute middleware already applied.
func RegisterRelayRoutes(relayRouter *gin.RouterGroup) {
	// Codex CLI protocol adapter: converts /v1/responses → /v1/chat/completions → /v1/responses
	relayRouter.POST("/codex/responses", protocol_adapter.HandleCodexResponses)
	relayRouter.POST("/codex/responses/compact", protocol_adapter.HandleCodexResponses)

	// Claude Code CLI protocol adapter: converts /v1/messages → /v1/chat/completions → /v1/messages
	// Note: The existing /v1/messages endpoint already handles this conversion natively.
	// This route provides an explicit entry point for the protocol adapter.
	relayRouter.POST("/claude/messages", protocol_adapter.HandleClaudeMessages)
}

// ResolveTokenVariables replaces ${token:name} placeholders in a header value
// using the internal token cache for the given userId.
func ResolveTokenVariables(value string, userId int) string {
	return token_config.ResolveTokenVariables(value, userId)
}

// ProxyFromEnvironmentWithWildcard is like http.ProxyFromEnvironment but
// supports wildcard patterns in NO_PROXY (e.g. *.huawei.com, 10.*).
func ProxyFromEnvironmentWithWildcard(req *http.Request) (*url.URL, error) {
	return proxyFromEnvironmentWithWildcard(req)
}
