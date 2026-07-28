// Copyright (C) 2023-2026 QuantumNous
// SPDX-License-Identifier: AGPL-3.0-or-later

package token_config

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
)

// GetTokenConfigs returns token configs for the current user.
// Admin users see all users' tokens; normal users see only their own.
func GetTokenConfigs(c *gin.Context) {
	userId := c.GetInt("id")
	role := c.GetInt("role")

	var configs []*TokenConfig
	var err error
	if role >= common.RoleAdminUser {
		configs, err = GetAllTokenConfigsFromDB()
	} else {
		configs, err = GetTokenConfigsByUserId(userId)
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}
	// Mask passwords in response
	for _, cfg := range configs {
		if cfg.Password != "" {
			cfg.Password = "***"
		}
	}

	// Check if admin allows token reveal
	revealAllowed := false
	common.OptionMapRWMutex.RLock()
	if val, ok := common.OptionMap["TokenRevealEnabled"]; ok {
		revealAllowed = val == "true" || val == "1"
	}
	common.OptionMapRWMutex.RUnlock()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    configs,
		"meta": gin.H{
			"reveal_allowed": revealAllowed,
		},
	})
}

// CreateTokenConfig creates token configs for the current user.
// For each token template that has login_url configured (including those referenced
// by channel templates via token_template_id), a TokenConfig is created.
// The user only provides username/password.
func CreateTokenConfig(c *gin.Context) {
	var input TokenConfig
	if err := c.ShouldBindJSON(&input); err != nil {
		common.ApiError(c, err)
		return
	}
	userId := c.GetInt("id")
	if input.Username == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "username is required",
		})
		return
	}

	// Collect unique token templates that need a TokenConfig:
	// 1. All token templates with login_url (standalone token templates)
	// 2. All token templates referenced by channel templates via token_template_id
	tokenTemplates, err := GetAllTokenTemplates()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	channelTemplates, err := GetAllChannelTemplates()
	if err != nil {
		common.ApiError(c, err)
		return
	}

	neededTemplateIds := map[int]bool{}
	for _, tmpl := range tokenTemplates {
		if tmpl.LoginURL != "" {
			neededTemplateIds[tmpl.Id] = true
		}
	}
	for _, ct := range channelTemplates {
		if ct.HasChannelTemplate() {
			tokenTplId := ct.GetTokenTemplateId()
			if tokenTplId > 0 {
				neededTemplateIds[tokenTplId] = true
			}
		}
	}

	var createdConfigs []TokenConfig
	for tplId := range neededTemplateIds {
		// Check duplicate
		existing, _ := GetTokenConfigByUsernameAndTemplateId(input.Username, tplId)
		if existing != nil {
			continue
		}

		cfg := TokenConfig{
			UserId:     userId,
			Username:   input.Username,
			Password:   input.Password,
			Enabled:    input.Enabled,
			TemplateId: tplId,
		}

		tmpl, err := GetTokenTemplateById(tplId)
		if err == nil && tmpl.LoginURL != "" {
			cfg.LoginURL = tmpl.LoginURL
			cfg.LoginMethod = tmpl.LoginMethod
			cfg.LoginHeaders = tmpl.LoginHeaders
			cfg.LoginBody = tmpl.LoginBody
			cfg.TokenJSONPath = tmpl.TokenJSONPath
			cfg.RefreshInterval = tmpl.RefreshInterval
		}

		if err := cfg.Insert(); err != nil {
			common.SysError(fmt.Sprintf("failed to create token config for template %d: %v", tplId, err))
			continue
		}
		createdConfigs = append(createdConfigs, cfg)
	}

	if len(createdConfigs) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "No new token configs created (may already exist)",
		})
		return
	}

	// Auto-create channels from all channel templates
	for _, ct := range channelTemplates {
		if !ct.HasChannelTemplate() {
			continue
		}
		channelId, err := autoCreateChannelFromChannelTemplate(createdConfigs[0], ct)
		if err != nil {
			common.SysError(fmt.Sprintf("failed to auto-create channel from channel template %d for user %s: %v", ct.Id, input.Username, err))
			continue
		}
		// Update first config's channel_id for display
		if createdConfigs[0].ChannelId == 0 {
			createdConfigs[0].ChannelId = channelId
			_ = db.Model(&createdConfigs[0]).Update("channel_id", channelId).Error
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    createdConfigs,
	})
}

// autoCreateChannelFromChannelTemplate clones the template channel (referenced by
// ct.ChannelTemplateId) with the user's token as the API key. The template channel
// is a disabled Channel that serves as a blueprint — admins edit it using the
// standard channel UI.
func autoCreateChannelFromChannelTemplate(cfg TokenConfig, ct *ChannelTemplate) (int, error) {
	if ChannelOps.CloneFromTemplate == nil {
		return 0, fmt.Errorf("channel operations not initialized")
	}
	return ChannelOps.CloneFromTemplate(ct.ChannelTemplateId, cfg.Username)
}

// UpdateTokenConfig updates an existing token config.
func UpdateTokenConfig(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "invalid id",
		})
		return
	}
	cfg, err := GetTokenConfigById(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "token config not found",
		})
		return
	}
	userId := c.GetInt("id")
	role := c.GetInt("role")
	if cfg.UserId != userId && role < common.RoleAdminUser {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "forbidden",
		})
		return
	}
	var input TokenConfig
	if err := c.ShouldBindJSON(&input); err != nil {
		common.ApiError(c, err)
		return
	}
	oldUsername := cfg.Username
	cfg.TemplateId = input.TemplateId
	cfg.LoginURL = input.LoginURL
	cfg.LoginMethod = input.LoginMethod
	cfg.LoginHeaders = input.LoginHeaders
	cfg.LoginBody = input.LoginBody
	cfg.Username = input.Username
	if input.Password != "" {
		cfg.Password = input.Password
	}
	cfg.TokenJSONPath = input.TokenJSONPath
	cfg.RefreshInterval = input.RefreshInterval
	cfg.Enabled = input.Enabled
	if err := cfg.Update(); err != nil {
		common.ApiError(c, err)
		return
	}

	// If username changed, update the auto-created channel name and key
	if oldUsername != cfg.Username && cfg.ChannelId > 0 {
		if ChannelOps.UpdateNameAndKey != nil {
			ChannelOps.UpdateNameAndKey(cfg.ChannelId, getTemplateName(cfg.TemplateId), cfg.Username)
		}
		// Migrate cache key
		DeleteTokenFromCache(oldUsername)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    cfg,
	})
}

// DeleteTokenConfig deletes a token config, its auto-created channel, and removes from cache.
func DeleteTokenConfig(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "invalid id",
		})
		return
	}
	cfg, err := GetTokenConfigById(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "token config not found",
		})
		return
	}
	userId := c.GetInt("id")
	role := c.GetInt("role")
	if cfg.UserId != userId && role < common.RoleAdminUser {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "forbidden",
		})
		return
	}

	// Delete the auto-created channel if it exists
	if cfg.ChannelId > 0 {
		if ChannelOps.Delete != nil {
			ChannelOps.Delete(cfg.ChannelId)
		}
	}

	if err := cfg.Delete(); err != nil {
		common.ApiError(c, err)
		return
	}
	DeleteTokenFromCache(cfg.Username)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}

// ManualRefreshToken forces a refresh of a specific token config.
func ManualRefreshToken(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "invalid id",
		})
		return
	}
	cfg, err := GetTokenConfigById(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "token config not found",
		})
		return
	}
	userId := c.GetInt("id")
	role := c.GetInt("role")
	if cfg.UserId != userId && role < common.RoleAdminUser {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "forbidden",
		})
		return
	}
	cfg, err = RefreshTokenConfig(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    cfg,
	})
}

// GetAllTokenConfigs returns all token configs across all users (admin only).
func GetAllTokenConfigs(c *gin.Context) {
	configs, err := GetAllTokenConfigsFromDB()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	// Mask passwords in response
	for _, cfg := range configs {
		if cfg.Password != "" {
			cfg.Password = "***"
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    configs,
	})
}

// getTemplateName returns the template name for a given template ID.
func getTemplateName(templateId int) string {
	tmpl, err := GetTokenTemplateById(templateId)
	if err != nil {
		return fmt.Sprintf("template-%d", templateId)
	}
	return tmpl.Name
}

// GetDisabledChannels returns disabled channels that can be used as channel templates.
// This is used by the template form to populate the channel template selector.
// The actual data comes from ChannelOps.GetDisabledChannels, injected by main.go.
func GetDisabledChannels(c *gin.Context) {
	if ChannelOps.GetDisabledChannels == nil {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    []interface{}{},
		})
		return
	}
	channels := ChannelOps.GetDisabledChannels()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    channels,
	})
}
