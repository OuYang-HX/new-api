// Copyright (C) 2023-2026 QuantumNous
// SPDX-License-Identifier: AGPL-3.0-or-later

package token_config

import (
	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// db is the GORM database instance, set via SetDB during initialization.
var db *gorm.DB

// SetDB sets the database instance for all token_config operations.
func SetDB(database *gorm.DB) {
	db = database
}

// TokenConfig stores a user's token auto-refresh configuration.
type TokenConfig struct {
	Id              int            `json:"id" gorm:"primaryKey;autoIncrement"`
	UserId          int            `json:"user_id" gorm:"index;not null"`
	Name            string         `json:"name" gorm:"size:128;not null;uniqueIndex:uk_token_config_name_user_del,priority:2"`
	LoginURL        string         `json:"login_url,omitempty" gorm:"type:text"`
	LoginMethod     string         `json:"login_method,omitempty" gorm:"size:16;default:'POST'"`
	LoginHeaders    string         `json:"login_headers,omitempty" gorm:"type:text"`
	LoginBody       string         `json:"login_body,omitempty" gorm:"type:text"`
	Username        string         `json:"username,omitempty" gorm:"size:256"`
	Password        string         `json:"password,omitempty" gorm:"size:256"`
	TokenJSONPath   string         `json:"token_json_path,omitempty" gorm:"size:256"`
	RefreshInterval int            `json:"refresh_interval" gorm:"default:3600"`
	CurrentToken    string         `json:"current_token,omitempty" gorm:"type:text"`
	TokenExpiresAt  int64          `json:"token_expires_at" gorm:"default:0"`
	Enabled         int            `json:"enabled" gorm:"default:1"`
	CreatedTime     int64          `json:"created_time" gorm:"bigint"`
	UpdatedTime     int64          `json:"updated_time" gorm:"bigint"`
	DeletedAt       gorm.DeletedAt `json:"-" gorm:"index;uniqueIndex:uk_token_config_name_user_del,priority:3"`
}

func (t *TokenConfig) Insert() error {
	now := common.GetTimestamp()
	t.CreatedTime = now
	t.UpdatedTime = now
	return db.Create(t).Error
}

func (t *TokenConfig) Update() error {
	t.UpdatedTime = common.GetTimestamp()
	return db.Save(t).Error
}

func (t *TokenConfig) Delete() error {
	return db.Delete(t).Error
}

func GetTokenConfigById(id int) (*TokenConfig, error) {
	var t TokenConfig
	err := db.First(&t, id).Error
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func GetTokenConfigsByUserId(userId int) ([]*TokenConfig, error) {
	var configs []*TokenConfig
	err := db.Where("user_id = ?", userId).Find(&configs).Error
	return configs, err
}

func GetAllEnabledTokenConfigs() ([]*TokenConfig, error) {
	var configs []*TokenConfig
	err := db.Where("enabled = 1").Find(&configs).Error
	return configs, err
}

func GetTokenConfigByNameAndUserId(name string, userId int) (*TokenConfig, error) {
	var t TokenConfig
	err := db.Where("name = ? AND user_id = ?", name, userId).First(&t).Error
	if err != nil {
		return nil, err
	}
	return &t, nil
}
