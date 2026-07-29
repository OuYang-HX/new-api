// Copyright (C) 2023-2026 QuantumNous
// SPDX-License-Identifier: AGPL-3.0-or-later

package token_config

import (
	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// ChannelTemplate stores an admin-defined channel blueprint template.
// It references a disabled Channel (the blueprint) and optionally a TokenTemplate
// that provides the login config for token refresh.
//
// When a user creates a TokenConfig, every ChannelTemplate will clone its
// referenced channel with the user's token as the API key, enabling self-service
// onboarding: users create their internal token → channels are auto-created →
// immediately usable.
//
// The channel template is a normal Channel with status=2 (manually disabled).
// Admins manage it using the standard channel editing UI, so any upstream
// changes to the Channel model are automatically reflected.
type ChannelTemplate struct {
	Id                int            `json:"id" gorm:"primaryKey;autoIncrement"`
	Name              string         `json:"name" gorm:"size:128;not null;uniqueIndex:uk_channel_template_name_del,priority:1"`
	ChannelTemplateId int            `json:"channel_template_id" gorm:"default:0"`
	TokenTemplateId   int            `json:"token_template_id" gorm:"default:0"`
	CreatedTime       int64          `json:"created_time" gorm:"bigint"`
	UpdatedTime       int64          `json:"updated_time" gorm:"bigint"`
	DeletedAt         gorm.DeletedAt `json:"-" gorm:"index;uniqueIndex:uk_channel_template_name_del,priority:2"`
}

func (t *ChannelTemplate) Insert() error {
	now := common.GetTimestamp()
	t.CreatedTime = now
	t.UpdatedTime = now
	return db.Create(t).Error
}

func (t *ChannelTemplate) Update() error {
	t.UpdatedTime = common.GetTimestamp()
	return db.Save(t).Error
}

func (t *ChannelTemplate) Delete() error {
	return db.Delete(t).Error
}

func GetChannelTemplateById(id int) (*ChannelTemplate, error) {
	var t ChannelTemplate
	err := db.First(&t, id).Error
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func GetAllChannelTemplates() ([]*ChannelTemplate, error) {
	var templates []*ChannelTemplate
	err := db.Find(&templates).Error
	return templates, err
}

// GetChannelTemplatesByChannelTemplateId returns all channel templates that
// reference the given disabled channel (blueprint) ID.
func GetChannelTemplatesByChannelTemplateId(channelTemplateId int) ([]*ChannelTemplate, error) {
	var templates []*ChannelTemplate
	err := db.Where("channel_template_id = ?", channelTemplateId).Find(&templates).Error
	return templates, err
}

// GetChannelTemplatesByTokenTemplateId returns all channel templates that use
// the given token template as their token source.
func GetChannelTemplatesByTokenTemplateId(tokenTemplateId int) ([]*ChannelTemplate, error) {
	var templates []*ChannelTemplate
	err := db.Where("token_template_id = ?", tokenTemplateId).Find(&templates).Error
	return templates, err
}

// HasChannelTemplate returns true if this template references a channel template
// that should be cloned when creating TokenConfigs.
func (t *ChannelTemplate) HasChannelTemplate() bool {
	return t.ChannelTemplateId > 0
}

// GetTokenTemplateId returns the template ID to use for token resolution.
// If TokenTemplateId is 0, returns 0 (caller decides fallback).
func (t *ChannelTemplate) GetTokenTemplateId() int {
	return t.TokenTemplateId
}
