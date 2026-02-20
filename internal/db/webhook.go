package db

import (
	"github.com/nickalie/nclaw/internal/model"
	"gorm.io/gorm"
)

// CreateWebhook inserts a new webhook registration.
func CreateWebhook(database *gorm.DB, webhook *model.WebhookRegistration) error {
	return database.Create(webhook).Error
}

// GetWebhookByID retrieves a single webhook by ID.
func GetWebhookByID(database *gorm.DB, id string) (*model.WebhookRegistration, error) {
	var webhook model.WebhookRegistration
	if err := database.First(&webhook, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &webhook, nil
}

// ListWebhooksByChat returns all webhooks for a given chat and thread.
func ListWebhooksByChat(database *gorm.DB, chatID int64, threadID int) ([]model.WebhookRegistration, error) {
	var webhooks []model.WebhookRegistration
	err := database.Where("chat_id = ? AND thread_id = ?", chatID, threadID).Find(&webhooks).Error
	return webhooks, err
}

// DeleteWebhook removes a webhook by ID.
func DeleteWebhook(database *gorm.DB, id string) error {
	return database.Where("id = ?", id).Delete(&model.WebhookRegistration{}).Error
}

// UpdateWebhookStatus sets the status of a webhook.
func UpdateWebhookStatus(database *gorm.DB, id, status string) error {
	return database.Model(&model.WebhookRegistration{}).Where("id = ?", id).Update("status", status).Error
}
