// Copyright (C) 2023-2026 QuantumNous
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package custom provides decoupled extensions to the upstream new-api.
// All custom features are registered here, keeping upstream files minimally modified.
package custom

import (
	"net/http"
	"net/url"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/custom/protocol_adapter"
	"github.com/QuantumNous/new-api/custom/token_config"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// RouteHandlers holds function references to controller handlers that custom routes need.
// This avoids importing the controller package (which would create import cycles).
// The caller (router) sets these before calling RegisterRoutes.
var RouteHandlers struct {
	// Codex channel credential refresh (gin handler wrapper)
	RefreshCodexChannelCredential func(c *gin.Context)
	// Codex OAuth
	StartCodexOAuth           func(c *gin.Context)
	CompleteCodexOAuth        func(c *gin.Context)
	StartCodexOAuthForChannel func(c *gin.Context)
	CompleteCodexOAuthForChannel func(c *gin.Context)
	GetCodexChannelUsage      func(c *gin.Context)
	// Custom OAuth Provider CRUD
	FetchCustomOAuthDiscovery  func(c *gin.Context)
	GetCustomOAuthProviders    func(c *gin.Context)
	GetCustomOAuthProvider     func(c *gin.Context)
	CreateCustomOAuthProvider  func(c *gin.Context)
	UpdateCustomOAuthProvider  func(c *gin.Context)
	DeleteCustomOAuthProvider  func(c *gin.Context)
	// User OAuth Binding
	GetUserOAuthBindings        func(c *gin.Context)
	GetUserOAuthBindingsByAdmin func(c *gin.Context)
	UnbindCustomOAuth           func(c *gin.Context)
	UnbindCustomOAuthByAdmin    func(c *gin.Context)
}

// SchedulerFuncs holds function references for custom schedulers.
// Set by main.go before calling StartSchedulers.
var SchedulerFuncs struct {
	// Codex credential auto-refresh
	StartCodexCredentialAutoRefreshTask func()
}

// MigrationModels holds model type references for custom migrations.
// Set by model/main.go before calling RegisterMigrations.
var MigrationModels struct {
	CustomOAuthProvider interface{}
	UserOAuthBinding    interface{}
}

// ChannelOperationFuncs holds function references for Channel CRUD operations.
// These are set by the caller (main.go) before calling RegisterMigrations
// to avoid import cycles between custom and model packages.
var ChannelOperationFuncs struct {
	CloneFromTemplate    func(channelTemplateId int, username string) (int, error)
	UpdateNameAndKey     func(channelId int, templateName string, username string)
	Delete               func(channelId int)
	GetById              func(channelId int) string
	SyncFromTemplate     func(channelTemplateId int, username string) error
	GetDisabledChannels  func() []token_config.DisabledChannelItem
}

// RegisterMigrations appends custom model migrations to the GORM AutoMigrate list.
// It also initializes the DB instance for custom packages.
func RegisterMigrations(database *gorm.DB) {
	token_config.SetDB(database)
	initChannelOps()
	database.AutoMigrate(&token_config.TokenConfig{})
	database.AutoMigrate(&token_config.TokenTemplate{})
	// Migration: drop NOT NULL on name column (replaced by username as unique identifier)
	// Only for PostgreSQL/MySQL; SQLite doesn't support ALTER COLUMN DROP NOT NULL
	if database.Migrator().HasColumn(&token_config.TokenConfig{}, "name") {
		if !common.UsingMainDatabase(common.DatabaseTypeSQLite) {
			_ = database.Exec("ALTER TABLE token_configs ALTER COLUMN name DROP NOT NULL").Error
		}
	}
	// custom-hook: Custom OAuth Provider and User OAuth Binding models
	if MigrationModels.CustomOAuthProvider != nil {
		database.AutoMigrate(MigrationModels.CustomOAuthProvider)
	}
	if MigrationModels.UserOAuthBinding != nil {
		database.AutoMigrate(MigrationModels.UserOAuthBinding)
	}
}

// RegisterMigrationsFast initializes the DB instance and returns custom models
// that should be added to the fast migration list. The caller appends them.
func RegisterMigrationsFast(database *gorm.DB) []interface{} {
	token_config.SetDB(database)
	initChannelOps()
	models := []interface{}{
		&token_config.TokenConfig{},
		&token_config.TokenTemplate{},
	}
	if MigrationModels.CustomOAuthProvider != nil {
		models = append(models, MigrationModels.CustomOAuthProvider)
	}
	if MigrationModels.UserOAuthBinding != nil {
		models = append(models, MigrationModels.UserOAuthBinding)
	}
	return models
}

// RegisterRoutes registers custom API routes on the given router groups.
// selfRoute is the user-protected self route group.
// adminRoute is the admin-protected user management route group.
// channelRoute is the admin-protected channel route group (may be nil if not yet created).
// rootRouter is the top-level API router (for root-only routes).
func RegisterRoutes(selfRoute *gin.RouterGroup, adminRoute *gin.RouterGroup, channelRoute *gin.RouterGroup, rootRouter *gin.RouterGroup) {
	// Re-inject channel ops in case they were not set during RegisterMigrations
	// (model.InitDB calls RegisterMigrations which calls initChannelOps before
	// main.go sets ChannelOperationFuncs)
	initChannelOps()

	// === Token Config routes ===
	tcRoute := selfRoute.Group("/token-config")
	tcRoute.GET("/", token_config.GetTokenConfigs)
	tcRoute.POST("/", token_config.CreateTokenConfig)
	tcRoute.PUT("/:id", token_config.UpdateTokenConfig)
	tcRoute.DELETE("/:id", token_config.DeleteTokenConfig)
	tcRoute.POST("/:id/refresh", token_config.ManualRefreshToken)

	// Token templates (admin-only CRUD, but also readable by users for selection)
	tcRoute.GET("/templates", token_config.GetTokenTemplates)
	tcRoute.GET("/disabled-channels", token_config.GetDisabledChannels)

	// Admin-only: CRUD templates
	adminRoute.POST("/token-config/templates", token_config.CreateTokenTemplate)
	adminRoute.PUT("/token-config/templates/:id", token_config.UpdateTokenTemplate)
	adminRoute.DELETE("/token-config/templates/:id", token_config.DeleteTokenTemplate)
	adminRoute.POST("/token-config/templates/:id/rebuild-channels", token_config.RebuildChannelsForTemplate)
	adminRoute.GET("/token-config/all", token_config.GetAllTokenConfigs)

	// === Codex OAuth routes (on channel route if available) ===
	if channelRoute != nil {
		channelRoute.POST("/codex/oauth/start", RouteHandlers.StartCodexOAuth)
		channelRoute.POST("/codex/oauth/complete", RouteHandlers.CompleteCodexOAuth)
		channelRoute.POST("/:id/codex/oauth/start", RouteHandlers.StartCodexOAuthForChannel)
		channelRoute.POST("/:id/codex/oauth/complete", RouteHandlers.CompleteCodexOAuthForChannel)
		if RouteHandlers.RefreshCodexChannelCredential != nil {
			channelRoute.POST("/:id/codex/refresh", RouteHandlers.RefreshCodexChannelCredential)
		}
		channelRoute.GET("/:id/codex/usage", RouteHandlers.GetCodexChannelUsage)
	}

	// === Custom OAuth Provider routes (root only) ===
	if rootRouter != nil {
		customOAuthRoute := rootRouter.Group("/custom-oauth-provider")
		if RouteHandlers.FetchCustomOAuthDiscovery != nil {
			customOAuthRoute.POST("/discovery", RouteHandlers.FetchCustomOAuthDiscovery)
		}
		if RouteHandlers.GetCustomOAuthProviders != nil {
			customOAuthRoute.GET("/", RouteHandlers.GetCustomOAuthProviders)
		}
		if RouteHandlers.GetCustomOAuthProvider != nil {
			customOAuthRoute.GET("/:id", RouteHandlers.GetCustomOAuthProvider)
		}
		if RouteHandlers.CreateCustomOAuthProvider != nil {
			customOAuthRoute.POST("/", RouteHandlers.CreateCustomOAuthProvider)
		}
		if RouteHandlers.UpdateCustomOAuthProvider != nil {
			customOAuthRoute.PUT("/:id", RouteHandlers.UpdateCustomOAuthProvider)
		}
		if RouteHandlers.DeleteCustomOAuthProvider != nil {
			customOAuthRoute.DELETE("/:id", RouteHandlers.DeleteCustomOAuthProvider)
		}
	}

	// === User OAuth Binding routes ===
	if selfRoute != nil && RouteHandlers.GetUserOAuthBindings != nil {
		selfRoute.GET("/oauth/bindings", RouteHandlers.GetUserOAuthBindings)
		if RouteHandlers.UnbindCustomOAuth != nil {
			selfRoute.DELETE("/oauth/bindings/:provider_id", RouteHandlers.UnbindCustomOAuth)
		}
	}
	if adminRoute != nil {
		if RouteHandlers.GetUserOAuthBindingsByAdmin != nil {
			adminRoute.GET("/:id/oauth/bindings", RouteHandlers.GetUserOAuthBindingsByAdmin)
		}
		if RouteHandlers.UnbindCustomOAuthByAdmin != nil {
			adminRoute.DELETE("/:id/oauth/bindings/:provider_id", RouteHandlers.UnbindCustomOAuthByAdmin)
		}
	}
}

// StartSchedulers launches custom background schedulers.
func StartSchedulers() {
	go token_config.StartTokenRefreshScheduler()
	// custom-hook: Codex credential auto-refresh
	if SchedulerFuncs.StartCodexCredentialAutoRefreshTask != nil {
		SchedulerFuncs.StartCodexCredentialAutoRefreshTask()
	}
}

// ResolveTokenVariables replaces ${token:name} placeholders in a header value
// using the internal token cache for the given userId.
func ResolveTokenVariables(value string, userId int) string {
	return token_config.ResolveTokenVariables(value, userId)
}

// initChannelOps injects the Channel operation functions into the token_config package.
// If the caller has set ChannelOperationFuncs via main.go, those are used.
// Otherwise, the functions remain nil and auto-channel-creation is a no-op.
func initChannelOps() {
	if ChannelOperationFuncs.CloneFromTemplate != nil {
		token_config.ChannelOps.CloneFromTemplate = ChannelOperationFuncs.CloneFromTemplate
	}
	if ChannelOperationFuncs.UpdateNameAndKey != nil {
		token_config.ChannelOps.UpdateNameAndKey = ChannelOperationFuncs.UpdateNameAndKey
	}
	if ChannelOperationFuncs.Delete != nil {
		token_config.ChannelOps.Delete = ChannelOperationFuncs.Delete
	}
	if ChannelOperationFuncs.GetById != nil {
		token_config.ChannelOps.GetById = ChannelOperationFuncs.GetById
	}
	if ChannelOperationFuncs.SyncFromTemplate != nil {
		token_config.ChannelOps.SyncFromTemplate = ChannelOperationFuncs.SyncFromTemplate
	}
	if ChannelOperationFuncs.GetDisabledChannels != nil {
		token_config.ChannelOps.GetDisabledChannels = ChannelOperationFuncs.GetDisabledChannels
	}
}

// ProxyFromEnvironmentWithWildcard is like http.ProxyFromEnvironment but
// supports wildcard patterns in NO_PROXY (e.g. *.huawei.com, 10.*).
func ProxyFromEnvironmentWithWildcard(req *http.Request) (*url.URL, error) {
	return proxyFromEnvironmentWithWildcard(req)
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
	// Codex CLI protocol adapter: /v1/codex/responses → chat/completions → Responses format
	relayRouter.POST("/codex/responses", protocol_adapter.HandleCodexResponses)
	relayRouter.POST("/codex/responses/compact", protocol_adapter.HandleCodexResponses)

	// Claude Code CLI protocol adapter: /v1/claude/messages
	relayRouter.POST("/claude/messages", protocol_adapter.HandleClaudeMessages)
}
// HandleCodexModels is a thin wrapper to expose HandleCodexModels without
// importing protocol_adapter from the router package (avoids pulling in
// unnecessary dependencies).
func HandleCodexModels(c *gin.Context) {
	protocol_adapter.HandleCodexModels(c)
}

// SyncChannelsFromChannelTemplate is called when a channel is updated via the admin UI.
// If the updated channel is used as a template by any TokenTemplate, all auto-created
// channels are synced with the updated fields.
func SyncChannelsFromChannelTemplate(channelId int) {
	if token_config.ChannelOps.SyncFromTemplate != nil {
		_ = token_config.ChannelOps.SyncFromTemplate(channelId, "")
	}
}
