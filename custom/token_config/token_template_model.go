// Copyright (C) 2023-2026 QuantumNous
// SPDX-License-Identifier: AGPL-3.0-or-later

package token_config

import (
	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// TokenTemplate stores an admin-defined template for token auto-refresh.
// Users reference a template when creating their own TokenConfig,
// only providing their credentials (username/password).
//
// When a template references a "channel template" (a disabled Channel that
// serves as a blueprint), creating a TokenConfig will automatically clone
// that channel with the user's token as the API key. This enables self-service
// onboarding: users create their internal token → a channel is auto-created →
// immediately usable.
//
// The channel template is a normal Channel with status=2 (manually disabled).
// Admins manage it using the standard channel editing UI, so any upstream
// changes to the Channel model are automatically reflected.
type TokenTemplate struct {
	Id              int            `json:"id" gorm:"primaryKey;autoIncrement"`
	Name            string         `json:"name" gorm:"size:128;not null;uniqueIndex:uk_token_template_name_del,priority:1"`
	LoginURL        string         `json:"login_url" gorm:"type:text"`
	LoginMethod     string         `json:"login_method" gorm:"size:16;default:'POST'"`
	LoginHeaders    string         `json:"login_headers" gorm:"type:text"`
	LoginBody       string         `json:"login_body" gorm:"type:text"`
	TokenJSONPath   string         `json:"token_json_path" gorm:"size:256"`
	RefreshInterval int            `json:"refresh_interval" gorm:"default:3600"`
	CreatedTime     int64          `json:"created_time" gorm:"bigint"`
	UpdatedTime     int64          `json:"updated_time" gorm:"bigint"`
	DeletedAt       gorm.DeletedAt `json:"-" gorm:"index;uniqueIndex:uk_token_template_name_del,priority:2"`

	// ChannelTemplateId references a disabled Channel that serves as the blueprint
	// for auto-creating per-user channels. When set and the referenced channel exists,
	// creating a TokenConfig from this template will clone that channel with:
	//   - Key replaced by ${token:<username>}
	//   - Name set to "<template_channel_name>-<username>"
	//   - Status set to enabled (1)
	//   - HeaderOverride: ${token:self} replaced with ${token:<username>}
	// When 0, no channel is auto-created.
	ChannelTemplateId int `json:"channel_template_id" gorm:"default:0"`

	// TokenTemplateId specifies which template's token to use for channel key resolution.
	// Points to a template that has login_url configured (a "token template").
	// When 0, this template's own login config is used if available.
	TokenTemplateId int `json:"token_template_id" gorm:"default:0"`
}

func (t *TokenTemplate) Insert() error {
	now := common.GetTimestamp()
	t.CreatedTime = now
	t.UpdatedTime = now
	return db.Create(t).Error
}

func (t *TokenTemplate) Update() error {
	t.UpdatedTime = common.GetTimestamp()
	return db.Save(t).Error
}

func (t *TokenTemplate) Delete() error {
	return db.Delete(t).Error
}

func GetTokenTemplateById(id int) (*TokenTemplate, error) {
	var t TokenTemplate
	err := db.First(&t, id).Error
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func GetAllTokenTemplates() ([]*TokenTemplate, error) {
	var templates []*TokenTemplate
	err := db.Find(&templates).Error
	return templates, err
}

// HasChannelTemplate returns true if this template references a channel template
// that should be cloned when creating TokenConfigs.
func (t *TokenTemplate) HasChannelTemplate() bool {
	return t.ChannelTemplateId > 0
}

// IsTokenTemplate returns true if this template provides login config for token refresh.
func (t *TokenTemplate) IsTokenTemplate() bool {
	return t.LoginURL != ""
}

// GetTokenTemplateId returns the template ID to use for token resolution.
// If TokenTemplateId is 0, defaults to self.
func (t *TokenTemplate) GetTokenTemplateId() int {
	if t.TokenTemplateId > 0 {
		return t.TokenTemplateId
	}
	return t.Id
}
