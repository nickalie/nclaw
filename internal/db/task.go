package db

import (
	"time"

	"github.com/nickalie/nclaw/internal/model"
	"gorm.io/gorm"
)

// CreateTask inserts a new scheduled task.
func CreateTask(database *gorm.DB, task *model.ScheduledTask) error {
	return database.Create(task).Error
}

// GetTask retrieves a single task by ID.
func GetTask(database *gorm.DB, id string) (*model.ScheduledTask, error) {
	var task model.ScheduledTask
	if err := database.First(&task, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

// ListTasksByChat returns all tasks for a given chat and thread.
func ListTasksByChat(database *gorm.DB, chatID int64, threadID int) ([]model.ScheduledTask, error) {
	var tasks []model.ScheduledTask
	err := database.Where("chat_id = ? AND thread_id = ?", chatID, threadID).Find(&tasks).Error
	return tasks, err
}

// GetDueTasks returns all active tasks whose next_run is at or before now.
func GetDueTasks(database *gorm.DB) ([]model.ScheduledTask, error) {
	var tasks []model.ScheduledTask
	err := database.Where("status = ? AND next_run IS NOT NULL AND next_run <= ?",
		model.StatusActive, time.Now()).
		Order("next_run").
		Find(&tasks).Error
	return tasks, err
}

// UpdateTaskStatus sets the status of a task.
func UpdateTaskStatus(database *gorm.DB, id, status string) error {
	return database.Model(&model.ScheduledTask{}).Where("id = ?", id).Update("status", status).Error
}

// UpdateTaskAfterRun updates last_run, last_result, next_run, and optionally status.
func UpdateTaskAfterRun(database *gorm.DB, id string, nextRun *time.Time, lastResult string) error {
	now := time.Now()
	updates := map[string]any{
		"last_run":    &now,
		"last_result": &lastResult,
		"next_run":    nextRun,
	}
	if nextRun == nil {
		updates["status"] = model.StatusCompleted
	}
	return database.Model(&model.ScheduledTask{}).Where("id = ?", id).Updates(updates).Error
}

// DeleteTask removes a task and its run logs.
func DeleteTask(database *gorm.DB, id string) error {
	database.Where("task_id = ?", id).Delete(&model.TaskRunLog{})
	return database.Where("id = ?", id).Delete(&model.ScheduledTask{}).Error
}
