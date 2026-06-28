// Copyright (C) 2023-2026 QuantumNous
// SPDX-License-Identifier: AGPL-3.0-or-later

package token_config

import (
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
	t.Name = input.Name
	t.LoginURL = input.LoginURL
	t.LoginMethod = input.LoginMethod
	t.LoginHeaders = input.LoginHeaders
	t.LoginBody = input.LoginBody
	t.TokenJSONPath = input.TokenJSONPath
	t.RefreshInterval = input.RefreshInterval
	if err := t.Update(); err != nil {
		common.ApiError(c, err)
		return
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
