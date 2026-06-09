// Copyright (C) 2023-2026 QuantumNous
// SPDX-License-Identifier: AGPL-3.0-or-later

package token_config

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
)

// GetTokenConfigs returns all token configs for the current user.
func GetTokenConfigs(c *gin.Context) {
	userId := c.GetInt("id")
	configs, err := GetTokenConfigsByUserId(userId)
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

// CreateTokenConfig creates a new token config for the current user.
func CreateTokenConfig(c *gin.Context) {
	var cfg TokenConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		common.ApiError(c, err)
		return
	}
	cfg.UserId = c.GetInt("id")
	if cfg.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "name is required",
		})
		return
	}
	if cfg.LoginURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "login_url is required",
		})
		return
	}
	if err := cfg.Insert(); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    cfg,
	})
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
	if cfg.UserId != userId {
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
	cfg.Name = input.Name
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
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    cfg,
	})
}

// DeleteTokenConfig deletes a token config and removes it from cache.
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
	if cfg.UserId != userId {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "forbidden",
		})
		return
	}
	if err := cfg.Delete(); err != nil {
		common.ApiError(c, err)
		return
	}
	DeleteTokenFromCache(cfg.UserId, cfg.Name)
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
	if cfg.UserId != userId {
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
