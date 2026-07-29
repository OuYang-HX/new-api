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

	// If channel template changed, sync all auto-created channels for the new blueprint.
	// Channels derived from the old blueprint are left for manual cleanup; their names
	// are based on the old blueprint name so they remain identifiable.
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

	// Delete all auto-created channels derived from this channel template.
	if t.HasChannelTemplate() && ChannelOps.DeleteChannelsForChannelTemplate != nil {
		ChannelOps.DeleteChannelsForChannelTemplate(t.ChannelTemplateId, t.TokenTemplateId)
	}

	if err := t.Delete(); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// RebuildChannelsForChannelTemplate creates channels for all TokenConfigs under this
// channel template that don't have a derived channel yet, and updates existing ones.
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

	if ChannelOps.SyncFromTemplate == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "channel operations not initialized"})
		return
	}

	if err := ChannelOps.SyncFromTemplate(t.ChannelTemplateId, ""); err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Channels rebuilt",
	})
}
