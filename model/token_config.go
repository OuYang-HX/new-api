package model

import (
	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

// TokenConfig 存储用户的令牌自动刷新配置
// Name 在同一用户下唯一（配合软删除）
// LoginURL / LoginMethod / LoginHeaders / LoginBody 描述如何获取令牌
// TokenJSONPath 用 JSONPath 从响应中提取令牌
// RefreshInterval 刷新间隔（秒），默认 3600
// CurrentToken / TokenExpiresAt 缓存当前令牌及过期时间
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

// Insert 创建新的令牌配置
func (t *TokenConfig) Insert() error {
	now := common.GetTimestamp()
	t.CreatedTime = now
	t.UpdatedTime = now
	return DB.Create(t).Error
}

// Update 更新令牌配置
func (t *TokenConfig) Update() error {
	t.UpdatedTime = common.GetTimestamp()
	return DB.Save(t).Error
}

// Delete 软删除令牌配置
func (t *TokenConfig) Delete() error {
	return DB.Delete(t).Error
}

// GetTokenConfigById 根据 ID 获取令牌配置
func GetTokenConfigById(id int) (*TokenConfig, error) {
	var t TokenConfig
	err := DB.First(&t, id).Error
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// GetTokenConfigsByUserId 获取指定用户的所有令牌配置
func GetTokenConfigsByUserId(userId int) ([]*TokenConfig, error) {
	var configs []*TokenConfig
	err := DB.Where("user_id = ?", userId).Find(&configs).Error
	return configs, err
}

// GetAllEnabledTokenConfigs 获取所有启用的令牌配置
func GetAllEnabledTokenConfigs() ([]*TokenConfig, error) {
	var configs []*TokenConfig
	err := DB.Where("enabled = 1").Find(&configs).Error
	return configs, err
}

// GetTokenConfigByNameAndUserId 根据名称和用户 ID 获取令牌配置
func GetTokenConfigByNameAndUserId(name string, userId int) (*TokenConfig, error) {
	var t TokenConfig
	err := DB.Where("name = ? AND user_id = ?", name, userId).First(&t).Error
	if err != nil {
		return nil, err
	}
	return &t, nil
}
