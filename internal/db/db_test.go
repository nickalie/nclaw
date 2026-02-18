package db

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/nickalie/nclaw/internal/model"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	database, err := Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, Init(database))
	return database
}

func sampleTask(chatID int64, threadID int) *model.ScheduledTask {
	now := time.Now().Add(time.Hour)
	return &model.ScheduledTask{
		ID:            model.GenerateTaskID(),
		ChatID:        chatID,
		ThreadID:      threadID,
		Prompt:        "test prompt",
		ScheduleType:  model.ScheduleInterval,
		ScheduleValue: "1h",
		ContextMode:   model.ContextGroup,
		Status:        model.StatusActive,
		NextRun:       &now,
		CreatedAt:     time.Now(),
	}
}

func TestOpenAndInit(t *testing.T) {
	database := setupTestDB(t)
	assert.NotNil(t, database)
}

func TestCreateAndGetTask(t *testing.T) {
	database := setupTestDB(t)
	task := sampleTask(123, 0)

	require.NoError(t, CreateTask(database, task))

	got, err := GetTask(database, task.ID)
	require.NoError(t, err)
	assert.Equal(t, task.ID, got.ID)
	assert.Equal(t, task.ChatID, got.ChatID)
	assert.Equal(t, task.Prompt, got.Prompt)
	assert.Equal(t, task.ScheduleType, got.ScheduleType)
	assert.Equal(t, task.Status, got.Status)
}

func TestGetTask_NotFound(t *testing.T) {
	database := setupTestDB(t)

	_, err := GetTask(database, "nonexistent")
	assert.Error(t, err)
}

func TestListTasksByChat(t *testing.T) {
	database := setupTestDB(t)

	task1 := sampleTask(100, 0)
	task2 := sampleTask(100, 0)
	task3 := sampleTask(200, 0) // different chat
	task4 := sampleTask(100, 5) // different thread

	for _, task := range []*model.ScheduledTask{task1, task2, task3, task4} {
		require.NoError(t, CreateTask(database, task))
	}

	tasks, err := ListTasksByChat(database, 100, 0)
	require.NoError(t, err)
	assert.Len(t, tasks, 2)

	tasks, err = ListTasksByChat(database, 200, 0)
	require.NoError(t, err)
	assert.Len(t, tasks, 1)

	tasks, err = ListTasksByChat(database, 100, 5)
	require.NoError(t, err)
	assert.Len(t, tasks, 1)

	tasks, err = ListTasksByChat(database, 999, 0)
	require.NoError(t, err)
	assert.Empty(t, tasks)
}

func TestGetDueTasks(t *testing.T) {
	database := setupTestDB(t)

	pastTime := time.Now().Add(-time.Hour)
	futureTime := time.Now().Add(time.Hour)

	due := sampleTask(100, 0)
	due.NextRun = &pastTime
	require.NoError(t, CreateTask(database, due))

	notDue := sampleTask(100, 0)
	notDue.NextRun = &futureTime
	require.NoError(t, CreateTask(database, notDue))

	paused := sampleTask(100, 0)
	paused.NextRun = &pastTime
	paused.Status = model.StatusPaused
	require.NoError(t, CreateTask(database, paused))

	tasks, err := GetDueTasks(database)
	require.NoError(t, err)
	assert.Len(t, tasks, 1)
	assert.Equal(t, due.ID, tasks[0].ID)
}

func TestUpdateTaskStatus(t *testing.T) {
	database := setupTestDB(t)
	task := sampleTask(100, 0)
	require.NoError(t, CreateTask(database, task))

	require.NoError(t, UpdateTaskStatus(database, task.ID, model.StatusPaused))

	got, err := GetTask(database, task.ID)
	require.NoError(t, err)
	assert.Equal(t, model.StatusPaused, got.Status)
}

func TestUpdateTaskAfterRun_WithNextRun(t *testing.T) {
	database := setupTestDB(t)
	task := sampleTask(100, 0)
	require.NoError(t, CreateTask(database, task))

	nextRun := time.Now().Add(2 * time.Hour)
	require.NoError(t, UpdateTaskAfterRun(database, task.ID, &nextRun, "done"))

	got, err := GetTask(database, task.ID)
	require.NoError(t, err)
	assert.NotNil(t, got.LastRun)
	assert.NotNil(t, got.LastResult)
	assert.Equal(t, "done", *got.LastResult)
	assert.Equal(t, model.StatusActive, got.Status) // not completed because nextRun is set
}

func TestUpdateTaskAfterRun_NilNextRun(t *testing.T) {
	database := setupTestDB(t)
	task := sampleTask(100, 0)
	require.NoError(t, CreateTask(database, task))

	require.NoError(t, UpdateTaskAfterRun(database, task.ID, nil, "final"))

	got, err := GetTask(database, task.ID)
	require.NoError(t, err)
	assert.Equal(t, model.StatusCompleted, got.Status) // completed because nextRun is nil
}

func TestDeleteTask(t *testing.T) {
	database := setupTestDB(t)
	task := sampleTask(100, 0)
	require.NoError(t, CreateTask(database, task))

	require.NoError(t, DeleteTask(database, task.ID))

	_, err := GetTask(database, task.ID)
	assert.Error(t, err)
}

func TestLogRun(t *testing.T) {
	database := setupTestDB(t)
	task := sampleTask(100, 0)
	require.NoError(t, CreateTask(database, task))

	result := "success result"
	entry := &model.TaskRunLog{
		TaskID:     task.ID,
		RunAt:      time.Now(),
		DurationMs: 1500,
		Status:     "success",
		Result:     &result,
	}

	require.NoError(t, LogRun(database, entry))
	assert.NotZero(t, entry.ID) // auto-incremented

	// Verify it was persisted
	var logs []model.TaskRunLog
	require.NoError(t, database.Where("task_id = ?", task.ID).Find(&logs).Error)
	assert.Len(t, logs, 1)
	assert.Equal(t, "success", logs[0].Status)
	assert.Equal(t, int64(1500), logs[0].DurationMs)
}
