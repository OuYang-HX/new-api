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

// GetChannelTemplates returns all channel templates (admin only).
func GetChannelTemplates(c *gin.Context) {
	templates, err := GetAllChannelTemplates()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    templates,
	})
}

// CreateChannelTemplate creates a new channel template (admin only).
func CreateChannelTemplate(c *gin.Context) {
	var t ChannelTemplate
	if err := c.ShouldBindJSON(&t); err != nil {
		common.ApiError(c, err)
		return
	}
	if t.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "name is required"})
		return
	}
	if !t.HasChannelTemplate() {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "channel_template_id is required"})
		return
	}
	if err := t.Insert(); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": t})
}

// UpdateChannelTemplate updates an existing channel template (admin only).
// If the template references a channel template and it changed, all auto-created
// channels are synced with the new channel template's fields.
func UpdateChannelTemplate(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid id"})
		return
	}
	t, err := GetChannelTemplateById(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "template not found"})
		return
	}
	var input ChannelTemplate
	if err := c.ShouldBindJSON(&input); err != nil {
		common.ApiError(c, err)
		return
	}
	oldChannelTemplateId := t.ChannelTemplateId
	t.Name = input.Name
	t.ChannelTemplateId = input.ChannelTemplateId
	t.TokenTemplateId = input.TokenTemplateId
	if err := t.Update(); err != nil {
		common.ApiError(c, err)
		return
	}

	// If channel template changed, sync all auto-created channels
	if t.HasChannelTemplate() && ChannelOps.SyncFromTemplate != nil {
		if t.ChannelTemplateId != oldChannelTemplateId || input.ChannelTemplateId > 0 {
			if err := ChannelOps.SyncFromTemplate(t.ChannelTemplateId, ""); err != nil {
				common.SysError(fmt.Sprintf("failed to sync channels from channel template %d: %v", t.Id, err))
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": t})
}

// DeleteChannelTemplate deletes a channel template (admin only).
// All auto-created channels linked to this template are also deleted.
func DeleteChannelTemplate(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid id"})
		return
	}
	t, err := GetChannelTemplateById(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "template not found"})
		return
	}

	// Delete all auto-created channels linked to this template
	if t.HasChannelTemplate() && ChannelOps.Delete != nil {
		configs, err := GetAllTokenConfigsFromDB()
		if err == nil {
			for _, cfg := range configs {
				if cfg.ChannelId > 0 {
					// Check if this channel belongs to this template by matching template name pattern
					// Channel name format: "<template_name>-<username>"
					expectedPrefix := t.Name + "-"
					if name := ChannelOps.GetById(cfg.ChannelId); name != "" && len(name) > len(expectedPrefix) && name[:len(expectedPrefix)] == expectedPrefix {
						ChannelOps.Delete(cfg.ChannelId)
						_ = db.Model(cfg).Update("channel_id", 0).Error
					}
				}
			}
		}
	}

	if err := t.Delete(); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// RebuildChannelsForChannelTemplate creates channels for all TokenConfigs that don't
// have a channel yet under this channel template. Also updates existing channels
// with the template's current config.
func RebuildChannelsForChannelTemplate(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid id"})
		return
	}
	t, err := GetChannelTemplateById(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "template not found"})
		return
	}
	if !t.HasChannelTemplate() {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "template has no channel template configured"})
		return
	}

	// Get all TokenConfigs
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
				channelId, err := autoCreateChannelFromChannelTemplate(*cfg, t)
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
			channelId, err := autoCreateChannelFromChannelTemplate(*cfg, t)
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
