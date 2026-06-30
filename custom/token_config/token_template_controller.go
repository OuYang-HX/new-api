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

// GetTokenTemplates returns all token templates (admin only).
func GetTokenTemplates(c *gin.Context) {
	templates, err := GetAllTokenTemplates()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    templates,
	})
}

// CreateTokenTemplate creates a new token template (admin only).
func CreateTokenTemplate(c *gin.Context) {
	var t TokenTemplate
	if err := c.ShouldBindJSON(&t); err != nil {
		common.ApiError(c, err)
		return
	}
	if t.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "name is required"})
		return
	}
	if t.LoginURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "login_url is required"})
		return
	}
	if err := t.Insert(); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": t})
}

// UpdateTokenTemplate updates an existing token template (admin only).
// If the template references a channel template and it changed, all auto-created
// channels are synced with the new channel template's fields.
func UpdateTokenTemplate(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid id"})
		return
	}
	t, err := GetTokenTemplateById(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "template not found"})
		return
	}
	var input TokenTemplate
	if err := c.ShouldBindJSON(&input); err != nil {
		common.ApiError(c, err)
		return
	}
	oldChannelTemplateId := t.ChannelTemplateId
	t.Name = input.Name
	t.LoginURL = input.LoginURL
	t.LoginMethod = input.LoginMethod
	t.LoginHeaders = input.LoginHeaders
	t.LoginBody = input.LoginBody
	t.TokenJSONPath = input.TokenJSONPath
	t.RefreshInterval = input.RefreshInterval
	t.ChannelTemplateId = input.ChannelTemplateId
	if err := t.Update(); err != nil {
		common.ApiError(c, err)
		return
	}

	// If channel template changed, sync all auto-created channels
	if t.HasChannelTemplate() && ChannelOps.SyncFromTemplate != nil {
		if t.ChannelTemplateId != oldChannelTemplateId || input.ChannelTemplateId > 0 {
			if err := ChannelOps.SyncFromTemplate(t.ChannelTemplateId, ""); err != nil {
				common.SysError(fmt.Sprintf("failed to sync channels from template %d: %v", t.Id, err))
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": t})
}

// DeleteTokenTemplate deletes a token template (admin only).
func DeleteTokenTemplate(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid id"})
		return
	}
	t, err := GetTokenTemplateById(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "template not found"})
		return
	}
	if err := t.Delete(); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// RebuildChannelsForTemplate creates channels for all TokenConfigs that belong to this template
// but don't have a channel yet. Also updates existing channels with the template's current config.
func RebuildChannelsForTemplate(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid id"})
		return
	}
	t, err := GetTokenTemplateById(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "template not found"})
		return
	}
	if !t.HasChannelTemplate() {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "template has no channel template configured"})
		return
	}

	// Get all TokenConfigs (template_id is 0 since users don't select a template)
	configs, err := GetAllTokenConfigsFromDB()
	if err != nil {
		common.ApiError(c, err)
		return
	}

	created := 0
	updated := 0
	for _, cfg := range configs {
		if cfg.ChannelId > 0 {
			// Check if channel still exists in DB
			var count int64
			if err := db.Table("channels").Where("id = ?", cfg.ChannelId).Count(&count).Error; err != nil || count == 0 {
				// Channel was deleted externally, recreate it
				cfg.ChannelId = 0
				channelId, err := autoCreateChannelFromTemplate(*cfg, t)
				if err != nil {
					common.SysError(fmt.Sprintf("failed to create channel for token config %d: %v", cfg.Id, err))
				} else {
					cfg.ChannelId = channelId
					_ = db.Model(cfg).Update("channel_id", channelId).Error
					created++
				}
			} else {
				// Channel exists, sync it
				if ChannelOps.SyncFromTemplate != nil {
					_ = ChannelOps.SyncFromTemplate(t.ChannelTemplateId, cfg.Username)
				}
				updated++
			}
		} else {
			// No channel yet, create one
			channelId, err := autoCreateChannelFromTemplate(*cfg, t)
			if err != nil {
				common.SysError(fmt.Sprintf("failed to create channel for token config %d: %v", cfg.Id, err))
			} else {
				cfg.ChannelId = channelId
				_ = db.Model(cfg).Update("channel_id", channelId).Error
				created++
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Created %d channels, updated %d channels", created, updated),
		"data": gin.H{
			"created": created,
			"updated": updated,
		},
	})
}
