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
	database.AutoMigrate(&token_config.TokenTemplate{})
}

// RegisterMigrationsFast initializes the DB instance and returns custom models
// that should be added to the fast migration list. The caller appends them.
func RegisterMigrationsFast(database *gorm.DB) []interface{} {
	token_config.SetDB(database)
	return []interface{}{&token_config.TokenConfig{}, &token_config.TokenTemplate{}}
}

// RegisterRoutes registers custom API routes on the given router group.
func RegisterRoutes(userRoute *gin.RouterGroup, adminRoute *gin.RouterGroup) {
	tcRoute := userRoute.Group("/token-config")
	tcRoute.GET("/", token_config.GetTokenConfigs)
	tcRoute.POST("/", token_config.CreateTokenConfig)
	tcRoute.PUT("/:id", token_config.UpdateTokenConfig)
	tcRoute.DELETE("/:id", token_config.DeleteTokenConfig)
	tcRoute.POST("/:id/refresh", token_config.ManualRefreshToken)

	// Token templates (admin-only CRUD, but also readable by users for selection)
	tcRoute.GET("/templates", token_config.GetTokenTemplates)
	adminRoute.POST("/token-config/templates", token_config.CreateTokenTemplate)
	adminRoute.PUT("/token-config/templates/:id", token_config.UpdateTokenTemplate)
	adminRoute.DELETE("/token-config/templates/:id", token_config.DeleteTokenTemplate)

	// Admin-only: get all token configs across users (for channel token picker)
	adminRoute.GET("/token-config/all", token_config.GetAllTokenConfigs)
}

// StartSchedulers launches custom background schedulers.
func StartSchedulers() {
	go token_config.StartTokenRefreshScheduler()
}

// InitProtocolAdapter initializes the protocol adapter by injecting the relay function
// and the enabled models function. This must be called before RegisterRelayRoutes
// to avoid nil function calls.
func InitProtocolAdapter(relayFunc func(c *gin.Context, relayFormat types.RelayFormat), enabledModelsFunc func() []string) {
	protocol_adapter.SetRelayFunc(relayFunc)
	protocol_adapter.SetEnabledModelsFn(enabledModelsFunc)
}

// RegisterRelayRoutes registers custom relay routes on the given router group.
// The router group should have TokenAuth and Distribute middleware already applied.
func RegisterRelayRoutes(relayRouter *gin.RouterGroup) {
	// Codex CLI protocol adapter: converts /v1/responses → /v1/chat/completions → /v1/responses
	relayRouter.POST("/codex/responses", protocol_adapter.HandleCodexResponses)
	relayRouter.POST("/codex/responses/compact", protocol_adapter.HandleCodexResponses)

	// Claude Code CLI protocol adapter: converts /v1/messages → /v1/chat/completions → /v1/messages
	relayRouter.POST("/claude/messages", protocol_adapter.HandleClaudeMessages)
}

// HandleCodexModels is a thin wrapper to expose HandleCodexModels without
// importing protocol_adapter from the router package (avoids pulling in
// unnecessary dependencies).
func HandleCodexModels(c *gin.Context) {
	protocol_adapter.HandleCodexModels(c)
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
