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

// CreateTokenConfig creates a new token config for the current user.
// The user provides username/password. Login config is inherited from the first
// template that has login_url configured. Channels are auto-created from all
// templates that have channel_template_id set.
func CreateTokenConfig(c *gin.Context) {
	var cfg TokenConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		common.ApiError(c, err)
		return
	}
	cfg.UserId = c.GetInt("id")
	if cfg.Username == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "username is required",
		})
		return
	}

	// Check for duplicate username across all templates
	existing, err := GetTokenConfigByUsername(cfg.Username)
	if err == nil && existing != nil {
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"message": fmt.Sprintf("username %q already exists", cfg.Username),
		})
		return
	}

	// Inherit login config from the first template that has login_url
	tmplId := cfg.TemplateId
	if tmplId == 0 {
		templates, err := GetAllTokenTemplates()
		if err == nil {
			for _, tmpl := range templates {
				if tmpl.LoginURL != "" {
					tmplId = tmpl.Id
					break
				}
			}
		}
	}
	if tmplId > 0 {
		tmpl, err := GetTokenTemplateById(tmplId)
		if err == nil {
			cfg.LoginURL = tmpl.LoginURL
			cfg.LoginMethod = tmpl.LoginMethod
			cfg.LoginHeaders = tmpl.LoginHeaders
			cfg.LoginBody = tmpl.LoginBody
			cfg.TokenJSONPath = tmpl.TokenJSONPath
			cfg.RefreshInterval = tmpl.RefreshInterval
		}
	}
	cfg.TemplateId = tmplId

	if err := cfg.Insert(); err != nil {
		common.ApiError(c, err)
		return
	}

	// Auto-create channels from all templates that have channel_template_id set
	allTemplates, err := GetAllTokenTemplates()
	if err == nil {
		for _, tmpl := range allTemplates {
			if tmpl.HasChannelTemplate() {
				channelId, err := autoCreateChannelFromTemplate(cfg, tmpl)
				if err != nil {
					common.SysError(fmt.Sprintf("failed to auto-create channel from template %d for user %s: %v", tmpl.Id, cfg.Username, err))
				} else {
					if cfg.ChannelId == 0 {
						cfg.ChannelId = channelId
					}
				}
			}
		}
	}
	if cfg.ChannelId > 0 {
		_ = db.Model(&cfg).Update("channel_id", cfg.ChannelId).Error
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    cfg,
	})
}

// autoCreateChannelFromTemplate clones the template channel (referenced by tmpl.ChannelTemplateId)
// with the user's token as the API key. The template channel is a disabled Channel that
// serves as a blueprint — admins edit it using the standard channel UI.
func autoCreateChannelFromTemplate(cfg TokenConfig, tmpl *TokenTemplate) (int, error) {
	if ChannelOps.CloneFromTemplate == nil {
		return 0, fmt.Errorf("channel operations not initialized")
	}
	return ChannelOps.CloneFromTemplate(tmpl.ChannelTemplateId, cfg.Username)
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
