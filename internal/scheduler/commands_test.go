package scheduler

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/nickalie/nclaw/internal/model"
	"github.com/nickalie/nclaw/internal/telegram"
)

func setupTestScheduler(t *testing.T) *Scheduler {
	t.Helper()
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&model.ScheduledTask{}, &model.TaskRunLog{}))

	sched, err := New(database, nil, nil, "UTC", t.TempDir(), telegram.NewChatLocker())
	require.NoError(t, err)
	return sched
}

func TestScheduleBlockRegex(t *testing.T) {
	input := "text\n```nclaw:schedule\n{\"action\":\"create\"}\n```\nmore"
	matches := scheduleBlockRe.FindAllStringSubmatch(input, -1)
	assert.Len(t, matches, 1)
	assert.Equal(t, "{\"action\":\"create\"}", matches[0][1])
}

func TestScheduleBlockRegex_Multiple(t *testing.T) {
	input := "```nclaw:schedule\n{\"action\":\"create\"}\n```\nmiddle\n```nclaw:schedule\n{\"action\":\"cancel\"}\n```"
	matches := scheduleBlockRe.FindAllStringSubmatch(input, -1)
	assert.Len(t, matches, 2)
}

func TestScheduleBlockRegex_NoMatch(t *testing.T) {
	input := "just regular text\n```go\nfmt.Println(\"hello\")\n```"
	matches := scheduleBlockRe.FindAllStringSubmatch(input, -1)
	assert.Empty(t, matches)
}

func TestProcessReply_NoBlocks(t *testing.T) {
	s := setupTestScheduler(t)
	result := s.ProcessReply("plain reply", 100, 0)
	assert.Equal(t, "plain reply", result)
}

func TestProcessReply_StripBlock(t *testing.T) {
	s := setupTestScheduler(t)
	reply := "before\n```nclaw:schedule\n{\"action\":\"cancel\",\"task_id\":\"nonexistent\"}\n```\nafter"
	result := s.ProcessReply(reply, 100, 0)
	// Block should be stripped, error appended
	assert.NotContains(t, result, "```nclaw:schedule")
	assert.Contains(t, result, "before")
	assert.Contains(t, result, "after")
}

func TestProcessReply_CreateTask(t *testing.T) {
	s := setupTestScheduler(t)
	s.Start()
	defer s.Shutdown()

	reply := "I'll set that up.\n```nclaw:schedule\n" +
		`{"action":"create","prompt":"check weather","type":"interval","value":"1h"}` +
		"\n```\nDone!"
	result := s.ProcessReply(reply, 100, 5)

	assert.Contains(t, result, "I'll set that up.")
	assert.Contains(t, result, "Done!")
	assert.NotContains(t, result, "nclaw:schedule")

	// Verify task was actually created in DB
	var tasks []model.ScheduledTask
	require.NoError(t, s.db.Find(&tasks).Error)
	assert.Len(t, tasks, 1)
	assert.Equal(t, "check weather", tasks[0].Prompt)
	assert.Equal(t, model.ScheduleInterval, tasks[0].ScheduleType)
	assert.Equal(t, "1h", tasks[0].ScheduleValue)
	assert.Equal(t, int64(100), tasks[0].ChatID)
	assert.Equal(t, 5, tasks[0].ThreadID)
}

func TestProcessReply_CreateTaskMissingFields(t *testing.T) {
	s := setupTestScheduler(t)
	reply := "```nclaw:schedule\n{\"action\":\"create\",\"prompt\":\"\"}\n```"
	result := s.ProcessReply(reply, 100, 0)
	assert.Contains(t, result, "Schedule error")
}

func TestProcessReply_InvalidJSON(t *testing.T) {
	s := setupTestScheduler(t)
	reply := "```nclaw:schedule\n{invalid json}\n```"
	result := s.ProcessReply(reply, 100, 0)
	assert.Contains(t, result, "Schedule error")
}

func TestProcessReply_UnknownAction(t *testing.T) {
	s := setupTestScheduler(t)
	reply := "```nclaw:schedule\n{\"action\":\"explode\"}\n```"
	result := s.ProcessReply(reply, 100, 0)
	assert.Contains(t, result, "Schedule error")
	assert.Contains(t, result, "unknown action")
}

func TestProcessReply_CreateWithContextMode(t *testing.T) {
	s := setupTestScheduler(t)
	s.Start()
	defer s.Shutdown()

	reply := "```nclaw:schedule\n" +
		`{"action":"create","prompt":"check","type":"interval","value":"30m","context":"isolated"}` +
		"\n```"
	result := s.ProcessReply(reply, 200, 0)
	assert.NotContains(t, result, "Schedule error")

	var tasks []model.ScheduledTask
	require.NoError(t, s.db.Find(&tasks).Error)
	assert.Equal(t, model.ContextIsolated, tasks[0].ContextMode)
}

func TestProcessReply_DefaultContextMode(t *testing.T) {
	s := setupTestScheduler(t)
	s.Start()
	defer s.Shutdown()

	reply := "```nclaw:schedule\n" +
		`{"action":"create","prompt":"check","type":"interval","value":"1h"}` +
		"\n```"
	s.ProcessReply(reply, 200, 0)

	var tasks []model.ScheduledTask
	require.NoError(t, s.db.Find(&tasks).Error)
	assert.Equal(t, model.ContextGroup, tasks[0].ContextMode)
}

func TestTruncate(t *testing.T) {
	assert.Equal(t, "hello", truncate("hello", 10))
	assert.Equal(t, "hello", truncate("hello", 5))
	assert.Equal(t, "hel", truncate("hello", 3))
	assert.Equal(t, "", truncate("", 5))
}

func TestFormatTaskList_Empty(t *testing.T) {
	s := setupTestScheduler(t)
	result := s.FormatTaskList(100, 0)
	assert.Equal(t, "Current scheduled tasks: none", result)
}

func TestFormatTaskList_WithTasks(t *testing.T) {
	s := setupTestScheduler(t)

	now := time.Now().Add(time.Hour)
	task := &model.ScheduledTask{
		ID:            "task-123",
		ChatID:        100,
		ThreadID:      0,
		Prompt:        "check weather",
		ScheduleType:  model.ScheduleInterval,
		ScheduleValue: "1h",
		ContextMode:   model.ContextGroup,
		Status:        model.StatusActive,
		NextRun:       &now,
		CreatedAt:     time.Now(),
	}
	require.NoError(t, s.db.Create(task).Error)

	result := s.FormatTaskList(100, 0)
	assert.Contains(t, result, "Current scheduled tasks:")
	assert.Contains(t, result, "task-123")
	assert.Contains(t, result, "check weather")
	assert.Contains(t, result, "interval")
	assert.Contains(t, result, "active")
}

func TestFormatTaskList_DifferentChat(t *testing.T) {
	s := setupTestScheduler(t)

	task := &model.ScheduledTask{
		ID:            "task-456",
		ChatID:        999,
		ThreadID:      0,
		Prompt:        "other task",
		ScheduleType:  model.ScheduleCron,
		ScheduleValue: "0 9 * * *",
		ContextMode:   model.ContextGroup,
		Status:        model.StatusActive,
		CreatedAt:     time.Now(),
	}
	require.NoError(t, s.db.Create(task).Error)

	// Should not appear for chat 100
	result := s.FormatTaskList(100, 0)
	assert.Equal(t, "Current scheduled tasks: none", result)
}
