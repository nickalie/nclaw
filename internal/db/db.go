package db

import (
	"github.com/nickalie/nclaw/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Open connects to the SQLite database at the given path with WAL mode.
func Open(path string) (*gorm.DB, error) {
	dsn := path + "?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=ON"
	return gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
}

// Init runs auto-migration for all models.
func Init(database *gorm.DB) error {
	return database.AutoMigrate(&model.ScheduledTask{}, &model.TaskRunLog{}, &model.WebhookRegistration{})
}
