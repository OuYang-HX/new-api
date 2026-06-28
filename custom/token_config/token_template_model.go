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
type TokenTemplate struct {
	Id              int            `json:"id" gorm:"primaryKey;autoIncrement"`
	Name            string         `json:"name" gorm:"size:128;not null;uniqueIndex:uk_token_template_name_del,priority:1"`
	LoginURL        string         `json:"login_url" gorm:"type:text;not null"`
	LoginMethod     string         `json:"login_method" gorm:"size:16;default:'POST'"`
	LoginHeaders    string         `json:"login_headers" gorm:"type:text"`
	LoginBody       string         `json:"login_body" gorm:"type:text"`
	TokenJSONPath   string         `json:"token_json_path" gorm:"size:256"`
	RefreshInterval int            `json:"refresh_interval" gorm:"default:3600"`
	CreatedTime     int64          `json:"created_time" gorm:"bigint"`
	UpdatedTime     int64          `json:"updated_time" gorm:"bigint"`
	DeletedAt       gorm.DeletedAt `json:"-" gorm:"index;uniqueIndex:uk_token_template_name_del,priority:2"`
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
