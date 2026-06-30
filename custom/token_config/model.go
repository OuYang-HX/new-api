// Copyright (C) 2023-2026 QuantumNous
// SPDX-License-Identifier: AGPL-3.0-or-later

package token_config

import (
	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// DisabledChannelItem is a lightweight representation of a disabled channel
// used in the template form's channel template selector.
type DisabledChannelItem struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
	Type int    `json:"type"`
}

// ChannelOperations holds function references for Channel CRUD operations.
// These are injected by the registry package to avoid import cycles between
// token_config and model packages.
var ChannelOps = struct {
	CloneFromTemplate    func(channelTemplateId int, username string) (int, error)
	UpdateNameAndKey     func(channelId int, templateName string, username string)
	Delete               func(channelId int)
	GetById              func(channelId int) string
	SyncFromTemplate     func(channelTemplateId int, username string) error
	GetDisabledChannels  func() []DisabledChannelItem
}{}

// db is the GORM database instance, set via SetDB during initialization.
var db *gorm.DB

// SetDB sets the database instance for all token_config operations.
func SetDB(database *gorm.DB) {
	db = database
}

// TokenConfig stores a user's token auto-refresh configuration.
// Username is the unique identifier (company account), replacing the old Name field.
// When the associated TokenTemplate has channel fields, creating a TokenConfig
// will automatically create a Channel.
type TokenConfig struct {
	Id              int            `json:"id" gorm:"primaryKey;autoIncrement"`
	UserId          int            `json:"user_id" gorm:"index;not null"`
	TemplateId      int            `json:"template_id" gorm:"index;not null;uniqueIndex:uk_token_config_template_username_del,priority:1"`
	Username        string         `json:"username" gorm:"size:256;not null;uniqueIndex:uk_token_config_template_username_del,priority:2"`
	Password        string         `json:"password,omitempty" gorm:"size:256"`
	LoginURL        string         `json:"login_url,omitempty" gorm:"type:text"`
	LoginMethod     string         `json:"login_method,omitempty" gorm:"size:16;default:'POST'"`
	LoginHeaders    string         `json:"login_headers,omitempty" gorm:"type:text"`
	LoginBody       string         `json:"login_body,omitempty" gorm:"type:text"`
	TokenJSONPath   string         `json:"token_json_path,omitempty" gorm:"size:256"`
	RefreshInterval int            `json:"refresh_interval" gorm:"default:3600"`
	CurrentToken    string         `json:"current_token,omitempty" gorm:"type:text"`
	TokenExpiresAt  int64          `json:"token_expires_at" gorm:"default:0"`
	Enabled         int            `json:"enabled" gorm:"default:1"`
	ChannelId       int            `json:"channel_id" gorm:"index;default:0"` // auto-created channel ID
	CreatedTime     int64          `json:"created_time" gorm:"bigint"`
	UpdatedTime     int64          `json:"updated_time" gorm:"bigint"`
	DeletedAt       gorm.DeletedAt `json:"-" gorm:"index;uniqueIndex:uk_token_config_template_username_del,priority:3"`
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

// GetTokenConfigByUsernameAndTemplateId returns the token config for a given username
// within a specific template. Username is the unique identifier per template.
func GetTokenConfigByUsernameAndTemplateId(username string, templateId int) (*TokenConfig, error) {
	var t TokenConfig
	err := db.Where("username = ? AND template_id = ?", username, templateId).First(&t).Error
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// GetTokenConfigByUsername returns the first enabled token config matching the given username
// across all templates. Used when resolving ${token:username} in channel configs.
func GetTokenConfigByUsername(username string) (*TokenConfig, error) {
	var t TokenConfig
	err := db.Where("username = ? AND enabled = 1", username).First(&t).Error
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// GetAllTokenConfigsFromDB returns all token configs (for admin use).
func GetAllTokenConfigsFromDB() ([]*TokenConfig, error) {
	var configs []*TokenConfig
	err := db.Find(&configs).Error
	return configs, err
}

// GetTokenConfigsByTemplateId returns all token configs for a given template.
func GetTokenConfigsByTemplateId(templateId int) ([]*TokenConfig, error) {
	var configs []*TokenConfig
	err := db.Where("template_id = ?", templateId).Find(&configs).Error
	return configs, err
}
