package db

import (
	"github.com/nickalie/nclaw/internal/model"
	"gorm.io/gorm"
)

// LogRun records a task execution result.
func LogRun(database *gorm.DB, entry *model.TaskRunLog) error {
	return database.Create(entry).Error
}
