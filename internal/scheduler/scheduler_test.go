package scheduler

import (
	"errors"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/nickalie/nclaw/internal/cli"
	"github.com/nickalie/nclaw/internal/model"
	"github.com/nickalie/nclaw/internal/pipeline"
	"github.com/nickalie/nclaw/internal/telegram"
)

func TestLoadTasks_OnceTaskPastTime(t *testing.T) {
	s := setupTestScheduler(t)
	s.Start()
	defer s.Shutdown()

	past := time.Now().Add(-time.Hour).UTC().Format("2006-01-02T15:04:05")
	task := &model.ScheduledTask{
		ID:            "task-past-once",
		ChatID:        100,
		Prompt:        "overdue task",
		ScheduleType:  model.ScheduleOnce,
		ScheduleValue: past,
		ContextMode:   model.ContextIsolated,
		Status:        model.StatusActive,
		CreatedAt:     time.Now(),
	}
	require.NoError(t, s.db.Create(task).Error)

	s.LoadTasks()

	s.mu.Lock()
	_, registered := s.jobs[task.ID]
	s.mu.Unlock()
	assert.True(t, registered, "once task with past time should be registered in gocron")
}

func TestLoadTasks_OnceTaskFutureTime(t *testing.T) {
	s := setupTestScheduler(t)
	s.Start()
	defer s.Shutdown()

	future := time.Now().Add(time.Hour).UTC().Format("2006-01-02T15:04:05")
	task := &model.ScheduledTask{
		ID:            "task-future-once",
		ChatID:        100,
		Prompt:        "future task",
		ScheduleType:  model.ScheduleOnce,
		ScheduleValue: future,
		ContextMode:   model.ContextIsolated,
		Status:        model.StatusActive,
		CreatedAt:     time.Now(),
	}
	require.NoError(t, s.db.Create(task).Error)

	s.LoadTasks()

	s.mu.Lock()
	_, registered := s.jobs[task.ID]
	s.mu.Unlock()
	assert.True(t, registered, "once task with future time should be registered in gocron")
}

func TestLoadTasks_SkipsPausedTasks(t *testing.T) {
	s := setupTestScheduler(t)
	s.Start()
	defer s.Shutdown()

	task := &model.ScheduledTask{
		ID:            "task-paused",
		ChatID:        100,
		Prompt:        "paused task",
		ScheduleType:  model.ScheduleInterval,
		ScheduleValue: "1h",
		ContextMode:   model.ContextGroup,
		Status:        model.StatusPaused,
		CreatedAt:     time.Now(),
	}
	require.NoError(t, s.db.Create(task).Error)

	s.LoadTasks()

	s.mu.Lock()
	_, registered := s.jobs[task.ID]
	s.mu.Unlock()
	assert.False(t, registered, "paused task should not be loaded")
}

func TestLoadTasks_MixedTasks(t *testing.T) {
	s := setupTestScheduler(t)
	s.Start()
	defer s.Shutdown()

	past := time.Now().Add(-time.Hour).UTC().Format("2006-01-02T15:04:05")
	tasks := []model.ScheduledTask{
		{
			ID: "task-cron", ChatID: 100, Prompt: "cron task",
			ScheduleType: model.ScheduleCron, ScheduleValue: "0 12 * * *",
			ContextMode: model.ContextGroup, Status: model.StatusActive, CreatedAt: time.Now(),
		},
		{
			ID: "task-interval", ChatID: 100, Prompt: "interval task",
			ScheduleType: model.ScheduleInterval, ScheduleValue: "30m",
			ContextMode: model.ContextGroup, Status: model.StatusActive, CreatedAt: time.Now(),
		},
		{
			ID: "task-once-past", ChatID: 100, Prompt: "overdue once",
			ScheduleType: model.ScheduleOnce, ScheduleValue: past,
			ContextMode: model.ContextIsolated, Status: model.StatusActive, CreatedAt: time.Now(),
		},
		{
			ID: "task-completed", ChatID: 100, Prompt: "done",
			ScheduleType: model.ScheduleInterval, ScheduleValue: "1h",
			ContextMode: model.ContextGroup, Status: model.StatusCompleted, CreatedAt: time.Now(),
		},
	}
	for i := range tasks {
		require.NoError(t, s.db.Create(&tasks[i]).Error)
	}

	s.LoadTasks()

	s.mu.Lock()
	defer s.mu.Unlock()
	assert.Contains(t, s.jobs, "task-cron")
	assert.Contains(t, s.jobs, "task-interval")
	assert.Contains(t, s.jobs, "task-once-past", "once task with past time should load via immediate mode")
	assert.NotContains(t, s.jobs, "task-completed", "completed task should not be loaded")
}

// --- PauseTask tests ---

func TestPauseTask_Active(t *testing.T) {
	s := setupTestScheduler(t)
	s.Start()
	defer s.Shutdown()

	task := &model.ScheduledTask{
		ID:            "task-pause-active",
		ChatID:        100,
		Prompt:        "pause me",
		ScheduleType:  model.ScheduleInterval,
		ScheduleValue: "1h",
		ContextMode:   model.ContextGroup,
		Status:        model.StatusActive,
		CreatedAt:     time.Now(),
	}
	require.NoError(t, s.db.Create(task).Error)
	require.NoError(t, s.addJob(task))

	err := s.PauseTask(task.ID)
	require.NoError(t, err)

	// Verify DB status is paused
	var updated model.ScheduledTask
	require.NoError(t, s.db.First(&updated, "id = ?", task.ID).Error)
	assert.Equal(t, model.StatusPaused, updated.Status)

	// Verify job removed from scheduler
	s.mu.Lock()
	_, registered := s.jobs[task.ID]
	s.mu.Unlock()
	assert.False(t, registered, "paused task should be removed from jobs map")
}

func TestPauseTask_AlreadyPaused(t *testing.T) {
	s := setupTestScheduler(t)

	task := &model.ScheduledTask{
		ID:            "task-pause-paused",
		ChatID:        100,
		Prompt:        "already paused",
		ScheduleType:  model.ScheduleInterval,
		ScheduleValue: "1h",
		ContextMode:   model.ContextGroup,
		Status:        model.StatusPaused,
		CreatedAt:     time.Now(),
	}
	require.NoError(t, s.db.Create(task).Error)

	err := s.PauseTask(task.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not active")
}

func TestPauseTask_NonExistent(t *testing.T) {
	s := setupTestScheduler(t)

	err := s.PauseTask("no-such-task")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get task")
}

// --- ResumeTask tests ---

func TestResumeTask_Paused(t *testing.T) {
	s := setupTestScheduler(t)
	s.Start()
	defer s.Shutdown()

	task := &model.ScheduledTask{
		ID:            "task-resume-paused",
		ChatID:        100,
		Prompt:        "resume me",
		ScheduleType:  model.ScheduleInterval,
		ScheduleValue: "1h",
		ContextMode:   model.ContextGroup,
		Status:        model.StatusPaused,
		CreatedAt:     time.Now(),
	}
	require.NoError(t, s.db.Create(task).Error)

	err := s.ResumeTask(task.ID)
	require.NoError(t, err)

	// Verify DB status is active
	var updated model.ScheduledTask
	require.NoError(t, s.db.First(&updated, "id = ?", task.ID).Error)
	assert.Equal(t, model.StatusActive, updated.Status)

	// Verify job registered in scheduler
	s.mu.Lock()
	_, registered := s.jobs[task.ID]
	s.mu.Unlock()
	assert.True(t, registered, "resumed task should be registered in jobs map")
}

func TestResumeTask_AlreadyActive(t *testing.T) {
	s := setupTestScheduler(t)
	s.Start()
	defer s.Shutdown()

	task := &model.ScheduledTask{
		ID:            "task-resume-active",
		ChatID:        100,
		Prompt:        "already active",
		ScheduleType:  model.ScheduleInterval,
		ScheduleValue: "1h",
		ContextMode:   model.ContextGroup,
		Status:        model.StatusActive,
		CreatedAt:     time.Now(),
	}
	require.NoError(t, s.db.Create(task).Error)

	err := s.ResumeTask(task.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not paused")
}

func TestResumeTask_NonExistent(t *testing.T) {
	s := setupTestScheduler(t)

	err := s.ResumeTask("no-such-task")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get task")
}

// --- CancelTask tests ---

func TestCancelTask_Active(t *testing.T) {
	s := setupTestScheduler(t)
	s.Start()
	defer s.Shutdown()

	task := &model.ScheduledTask{
		ID:            "task-cancel-active",
		ChatID:        100,
		Prompt:        "cancel me",
		ScheduleType:  model.ScheduleInterval,
		ScheduleValue: "1h",
		ContextMode:   model.ContextGroup,
		Status:        model.StatusActive,
		CreatedAt:     time.Now(),
	}
	require.NoError(t, s.db.Create(task).Error)
	require.NoError(t, s.addJob(task))

	err := s.CancelTask(task.ID)
	require.NoError(t, err)

	// Verify task deleted from DB
	var count int64
	s.db.Model(&model.ScheduledTask{}).Where("id = ?", task.ID).Count(&count)
	assert.Equal(t, int64(0), count, "canceled task should be deleted from DB")

	// Verify job removed from scheduler
	s.mu.Lock()
	_, registered := s.jobs[task.ID]
	s.mu.Unlock()
	assert.False(t, registered, "canceled task should be removed from jobs map")
}

func TestCancelTask_NonExistent(t *testing.T) {
	s := setupTestScheduler(t)

	// CancelTask calls db.DeleteTask which does not error on missing rows
	// (DELETE WHERE is a no-op), so this should succeed
	err := s.CancelTask("no-such-task")
	require.NoError(t, err)
}

// --- CreateTask tests ---

func TestCreateTask_Interval(t *testing.T) {
	s := setupTestScheduler(t)
	s.Start()
	defer s.Shutdown()

	task := &model.ScheduledTask{
		ID:            "task-create-interval",
		ChatID:        200,
		ThreadID:      1,
		Prompt:        "check status",
		ScheduleType:  model.ScheduleInterval,
		ScheduleValue: "30m",
		ContextMode:   model.ContextGroup,
		Status:        model.StatusActive,
		CreatedAt:     time.Now(),
	}

	err := s.CreateTask(task)
	require.NoError(t, err)

	// Verify persisted in DB
	var stored model.ScheduledTask
	require.NoError(t, s.db.First(&stored, "id = ?", task.ID).Error)
	assert.Equal(t, "check status", stored.Prompt)
	assert.Equal(t, model.ScheduleInterval, stored.ScheduleType)
	assert.Equal(t, "30m", stored.ScheduleValue)
	assert.Equal(t, int64(200), stored.ChatID)
	assert.Equal(t, 1, stored.ThreadID)

	// Verify registered in gocron
	s.mu.Lock()
	_, registered := s.jobs[task.ID]
	s.mu.Unlock()
	assert.True(t, registered)
}

func TestCreateTask_Cron(t *testing.T) {
	s := setupTestScheduler(t)
	s.Start()
	defer s.Shutdown()

	task := &model.ScheduledTask{
		ID:            "task-create-cron",
		ChatID:        200,
		Prompt:        "daily report",
		ScheduleType:  model.ScheduleCron,
		ScheduleValue: "0 9 * * *",
		ContextMode:   model.ContextIsolated,
		Status:        model.StatusActive,
		CreatedAt:     time.Now(),
	}

	err := s.CreateTask(task)
	require.NoError(t, err)

	var stored model.ScheduledTask
	require.NoError(t, s.db.First(&stored, "id = ?", task.ID).Error)
	assert.Equal(t, model.ScheduleCron, stored.ScheduleType)
	assert.Equal(t, model.ContextIsolated, stored.ContextMode)

	s.mu.Lock()
	_, registered := s.jobs[task.ID]
	s.mu.Unlock()
	assert.True(t, registered)
}

func TestCreateTask_Once(t *testing.T) {
	s := setupTestScheduler(t)
	s.Start()
	defer s.Shutdown()

	future := time.Now().Add(2 * time.Hour).UTC().Format("2006-01-02T15:04:05")
	task := &model.ScheduledTask{
		ID:            "task-create-once",
		ChatID:        200,
		Prompt:        "one-time reminder",
		ScheduleType:  model.ScheduleOnce,
		ScheduleValue: future,
		ContextMode:   model.ContextGroup,
		Status:        model.StatusActive,
		CreatedAt:     time.Now(),
	}

	err := s.CreateTask(task)
	require.NoError(t, err)

	var stored model.ScheduledTask
	require.NoError(t, s.db.First(&stored, "id = ?", task.ID).Error)
	assert.Equal(t, model.ScheduleOnce, stored.ScheduleType)

	s.mu.Lock()
	_, registered := s.jobs[task.ID]
	s.mu.Unlock()
	assert.True(t, registered)
}

// --- jobDefinition tests ---

func TestJobDefinition_Cron(t *testing.T) {
	s := setupTestScheduler(t)

	task := &model.ScheduledTask{
		ScheduleType:  model.ScheduleCron,
		ScheduleValue: "*/5 * * * *",
	}
	def, err := s.jobDefinition(task)
	require.NoError(t, err)
	assert.NotNil(t, def)
}

func TestJobDefinition_Interval(t *testing.T) {
	s := setupTestScheduler(t)

	task := &model.ScheduledTask{
		ScheduleType:  model.ScheduleInterval,
		ScheduleValue: "45m",
	}
	def, err := s.jobDefinition(task)
	require.NoError(t, err)
	assert.NotNil(t, def)
}

func TestJobDefinition_IntervalInvalid(t *testing.T) {
	s := setupTestScheduler(t)

	task := &model.ScheduledTask{
		ScheduleType:  model.ScheduleInterval,
		ScheduleValue: "not-a-duration",
	}
	_, err := s.jobDefinition(task)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse interval")
}

func TestJobDefinition_OnceFuture(t *testing.T) {
	s := setupTestScheduler(t)

	future := time.Now().Add(time.Hour).UTC().Format("2006-01-02T15:04:05")
	task := &model.ScheduledTask{
		ID:            "task-def-once-future",
		ScheduleType:  model.ScheduleOnce,
		ScheduleValue: future,
	}
	def, err := s.jobDefinition(task)
	require.NoError(t, err)
	assert.NotNil(t, def)
}

func TestJobDefinition_OncePast(t *testing.T) {
	s := setupTestScheduler(t)

	past := time.Now().Add(-time.Hour).UTC().Format("2006-01-02T15:04:05")
	task := &model.ScheduledTask{
		ID:            "task-def-once-past",
		ScheduleType:  model.ScheduleOnce,
		ScheduleValue: past,
	}
	def, err := s.jobDefinition(task)
	require.NoError(t, err)
	assert.NotNil(t, def, "past once task should return immediate job definition")
}

func TestJobDefinition_OnceInvalidTime(t *testing.T) {
	s := setupTestScheduler(t)

	task := &model.ScheduledTask{
		ScheduleType:  model.ScheduleOnce,
		ScheduleValue: "not-a-time",
	}
	_, err := s.jobDefinition(task)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse once time")
}

func TestJobDefinition_UnknownType(t *testing.T) {
	s := setupTestScheduler(t)

	task := &model.ScheduledTask{
		ScheduleType:  "lunar",
		ScheduleValue: "full-moon",
	}
	_, err := s.jobDefinition(task)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown schedule type")
}

// --- Start/Shutdown lifecycle test ---

func TestStartShutdown(t *testing.T) {
	s := setupTestScheduler(t)

	// Start should not panic
	s.Start()

	// Create a task while running to verify scheduler is functional
	task := &model.ScheduledTask{
		ID:            "task-lifecycle",
		ChatID:        100,
		Prompt:        "lifecycle test",
		ScheduleType:  model.ScheduleInterval,
		ScheduleValue: "1h",
		ContextMode:   model.ContextGroup,
		Status:        model.StatusActive,
		CreatedAt:     time.Now(),
	}
	err := s.CreateTask(task)
	require.NoError(t, err)

	s.mu.Lock()
	_, registered := s.jobs[task.ID]
	s.mu.Unlock()
	assert.True(t, registered)

	// Shutdown should not error
	err = s.Shutdown()
	require.NoError(t, err)
}

// --- Pause then Resume round-trip ---

func TestPauseThenResume(t *testing.T) {
	s := setupTestScheduler(t)
	s.Start()
	defer s.Shutdown()

	task := &model.ScheduledTask{
		ID:            "task-pause-resume",
		ChatID:        100,
		Prompt:        "round trip",
		ScheduleType:  model.ScheduleInterval,
		ScheduleValue: "2h",
		ContextMode:   model.ContextGroup,
		Status:        model.StatusActive,
		CreatedAt:     time.Now(),
	}
	require.NoError(t, s.db.Create(task).Error)
	require.NoError(t, s.addJob(task))

	// Pause
	require.NoError(t, s.PauseTask(task.ID))
	s.mu.Lock()
	_, registered := s.jobs[task.ID]
	s.mu.Unlock()
	assert.False(t, registered, "job should be removed after pause")

	// Resume
	require.NoError(t, s.ResumeTask(task.ID))
	s.mu.Lock()
	_, registered = s.jobs[task.ID]
	s.mu.Unlock()
	assert.True(t, registered, "job should be re-registered after resume")

	// Verify DB status is active again
	var updated model.ScheduledTask
	require.NoError(t, s.db.First(&updated, "id = ?", task.ID).Error)
	assert.Equal(t, model.StatusActive, updated.Status)
}

// --- Mock CLI client and provider for execution tests ---

type mockCLIClient struct{}

func (m *mockCLIClient) Dir(_ string) cli.Client                { return m }
func (m *mockCLIClient) SkipPermissions() cli.Client            { return m }
func (m *mockCLIClient) AppendSystemPrompt(_ string) cli.Client { return m }

func (m *mockCLIClient) Ask(_ string) (*cli.Result, error) {
	return &cli.Result{Text: "mock reply", FullText: "mock full reply"}, nil
}

func (m *mockCLIClient) Continue(_ string) (*cli.Result, error) {
	return &cli.Result{Text: "mock reply", FullText: "mock full reply"}, nil
}

type mockCLIProvider struct{}

func (m *mockCLIProvider) NewClient() cli.Client    { return &mockCLIClient{} }
func (m *mockCLIProvider) PreInvoke() error         { return nil }
func (m *mockCLIProvider) Version() (string, error) { return "mock-cli-1.0.0", nil }
func (m *mockCLIProvider) Name() string             { return "mock-cli" }

func setupTestSchedulerWithMockCLI(t *testing.T) *Scheduler {
	t.Helper()
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&model.ScheduledTask{}, &model.TaskRunLog{}))

	sched, err := New(database, &mockCLIProvider{}, "UTC", t.TempDir(), telegram.NewChatLocker())
	require.NoError(t, err)
	return sched
}

// --- Execution path tests ---

func TestSetPipeline(t *testing.T) {
	s := setupTestScheduler(t)
	assert.Nil(t, s.pipeline)

	p := &pipeline.Pipeline{}
	s.SetPipeline(p)
	assert.Equal(t, p, s.pipeline)
}

func TestExecuteTask_Success(t *testing.T) {
	s := setupTestSchedulerWithMockCLI(t)
	s.Start()
	defer s.Shutdown()

	task := &model.ScheduledTask{
		ID:            "exec-success",
		ChatID:        100,
		Prompt:        "do something",
		ScheduleType:  model.ScheduleInterval,
		ScheduleValue: "1h",
		ContextMode:   model.ContextGroup,
		Status:        model.StatusActive,
		CreatedAt:     time.Now(),
	}
	require.NoError(t, s.db.Create(task).Error)

	s.executeTask(task.ID)

	// Verify TaskRunLog was created with status=success
	var logs []model.TaskRunLog
	require.NoError(t, s.db.Where("task_id = ?", task.ID).Find(&logs).Error)
	require.Len(t, logs, 1)
	assert.Equal(t, "success", logs[0].Status)
	assert.Nil(t, logs[0].Error)
	assert.NotNil(t, logs[0].Result)
}

func TestExecuteTask_TaskNotFound(t *testing.T) {
	s := setupTestSchedulerWithMockCLI(t)

	// Should not panic; just returns without doing anything
	s.executeTask("nonexistent-task")

	// Verify no run logs were created
	var count int64
	s.db.Model(&model.TaskRunLog{}).Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestExecuteTask_InactiveTask(t *testing.T) {
	s := setupTestSchedulerWithMockCLI(t)

	task := &model.ScheduledTask{
		ID:            "exec-inactive",
		ChatID:        100,
		Prompt:        "paused task",
		ScheduleType:  model.ScheduleInterval,
		ScheduleValue: "1h",
		ContextMode:   model.ContextGroup,
		Status:        model.StatusPaused,
		CreatedAt:     time.Now(),
	}
	require.NoError(t, s.db.Create(task).Error)

	s.executeTask(task.ID)

	// Verify no run was logged
	var count int64
	s.db.Model(&model.TaskRunLog{}).Where("task_id = ?", task.ID).Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestRecordResults_Success(t *testing.T) {
	s := setupTestSchedulerWithMockCLI(t)
	s.Start()
	defer s.Shutdown()

	task := &model.ScheduledTask{
		ID:            "record-success",
		ChatID:        100,
		Prompt:        "test task",
		ScheduleType:  model.ScheduleInterval,
		ScheduleValue: "1h",
		ContextMode:   model.ContextGroup,
		Status:        model.StatusActive,
		CreatedAt:     time.Now(),
	}
	require.NoError(t, s.db.Create(task).Error)
	require.NoError(t, s.addJob(task))

	err := s.recordResults(task, "some reply", nil, 5*time.Second)
	require.NoError(t, err)

	// Verify TaskRunLog was created
	var logs []model.TaskRunLog
	require.NoError(t, s.db.Where("task_id = ?", task.ID).Find(&logs).Error)
	require.Len(t, logs, 1)
	assert.Equal(t, "success", logs[0].Status)

	// Verify task was updated
	var updated model.ScheduledTask
	require.NoError(t, s.db.First(&updated, "id = ?", task.ID).Error)
	assert.Contains(t, *updated.LastResult, "some reply")
}

func TestRecordResults_Error(t *testing.T) {
	s := setupTestSchedulerWithMockCLI(t)
	s.Start()
	defer s.Shutdown()

	task := &model.ScheduledTask{
		ID:            "record-error",
		ChatID:        100,
		Prompt:        "once task",
		ScheduleType:  model.ScheduleOnce,
		ScheduleValue: time.Now().Add(time.Hour).UTC().Format("2006-01-02T15:04:05"),
		ContextMode:   model.ContextIsolated,
		Status:        model.StatusActive,
		CreatedAt:     time.Now(),
	}
	require.NoError(t, s.db.Create(task).Error)
	require.NoError(t, s.addJob(task))

	runErr := errors.New("cli failed")
	err := s.recordResults(task, "", runErr, 2*time.Second)
	require.NoError(t, err)

	// Verify task status became "failed" for once task with error
	var updated model.ScheduledTask
	require.NoError(t, s.db.First(&updated, "id = ?", task.ID).Error)
	assert.Equal(t, model.StatusFailed, updated.Status)
}

func TestRecordResults_DeletedTask(t *testing.T) {
	s := setupTestSchedulerWithMockCLI(t)

	task := &model.ScheduledTask{
		ID:            "record-deleted",
		ChatID:        100,
		Prompt:        "gone task",
		ScheduleType:  model.ScheduleInterval,
		ScheduleValue: "1h",
		ContextMode:   model.ContextGroup,
		Status:        model.StatusActive,
		CreatedAt:     time.Now(),
	}
	// Don't create the task in DB — it's "deleted"

	err := s.recordResults(task, "reply", nil, time.Second)
	require.Error(t, err)
	assert.True(t, errors.Is(err, gorm.ErrRecordNotFound))
}

func TestLogRunTx_Success(t *testing.T) {
	s := setupTestSchedulerWithMockCLI(t)

	// Create a task first so foreign key (if any) is satisfied
	task := &model.ScheduledTask{
		ID:            "logrun-success",
		ChatID:        100,
		Prompt:        "test",
		ScheduleType:  model.ScheduleInterval,
		ScheduleValue: "1h",
		ContextMode:   model.ContextGroup,
		Status:        model.StatusActive,
		CreatedAt:     time.Now(),
	}
	require.NoError(t, s.db.Create(task).Error)

	err := s.logRunTx(s.db, task.ID, "reply text", nil, 3*time.Second)
	require.NoError(t, err)

	var logs []model.TaskRunLog
	require.NoError(t, s.db.Where("task_id = ?", task.ID).Find(&logs).Error)
	require.Len(t, logs, 1)
	assert.Equal(t, "success", logs[0].Status)
	assert.Nil(t, logs[0].Error)
	assert.NotNil(t, logs[0].Result)
	assert.Equal(t, "reply text", *logs[0].Result)
}

func TestLogRunTx_Error(t *testing.T) {
	s := setupTestSchedulerWithMockCLI(t)

	task := &model.ScheduledTask{
		ID:            "logrun-error",
		ChatID:        100,
		Prompt:        "test",
		ScheduleType:  model.ScheduleInterval,
		ScheduleValue: "1h",
		ContextMode:   model.ContextGroup,
		Status:        model.StatusActive,
		CreatedAt:     time.Now(),
	}
	require.NoError(t, s.db.Create(task).Error)

	runErr := errors.New("something went wrong")
	err := s.logRunTx(s.db, task.ID, "", runErr, 2*time.Second)
	require.NoError(t, err)

	var logs []model.TaskRunLog
	require.NoError(t, s.db.Where("task_id = ?", task.ID).Find(&logs).Error)
	require.Len(t, logs, 1)
	assert.Equal(t, "error", logs[0].Status)
	require.NotNil(t, logs[0].Error)
	assert.Equal(t, "something went wrong", *logs[0].Error)
}

func TestResolveNextRun_OnceTask(t *testing.T) {
	s := setupTestSchedulerWithMockCLI(t)

	task := &model.ScheduledTask{
		ID:           "resolve-once",
		ScheduleType: model.ScheduleOnce,
	}

	result := s.resolveNextRun(task)
	assert.Nil(t, result, "once tasks should return nil next run")
}

func TestResolveNextRun_IntervalTask(t *testing.T) {
	s := setupTestSchedulerWithMockCLI(t)
	s.Start()
	defer s.Shutdown()

	task := &model.ScheduledTask{
		ID:            "resolve-interval",
		ChatID:        100,
		Prompt:        "test",
		ScheduleType:  model.ScheduleInterval,
		ScheduleValue: "1h",
		ContextMode:   model.ContextGroup,
		Status:        model.StatusActive,
		CreatedAt:     time.Now(),
	}
	require.NoError(t, s.db.Create(task).Error)
	require.NoError(t, s.addJob(task))

	result := s.resolveNextRun(task)
	assert.NotNil(t, result, "interval task with registered job should have a next run time")
}

func TestGetNextRun_NoJobs(t *testing.T) {
	s := setupTestSchedulerWithMockCLI(t)
	s.Start()
	defer s.Shutdown()

	// Use a random UUID that doesn't match any job
	fakeID := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	result := s.getNextRun(fakeID)
	assert.Nil(t, result, "should return nil for unknown job ID")
}

func TestSendResult_NilPipeline(t *testing.T) {
	s := setupTestSchedulerWithMockCLI(t)
	// pipeline is nil by default

	task := &model.ScheduledTask{
		ID:       "send-nil-pipeline",
		ChatID:   100,
		ThreadID: 0,
	}
	result := &cli.Result{Text: "hello", FullText: "hello"}

	// Should not panic
	assert.NotPanics(t, func() {
		s.sendResult(task, result, nil)
	})
}

func TestClearRunState(t *testing.T) {
	s := setupTestSchedulerWithMockCLI(t)

	taskID := "clear-state-test"

	s.mu.Lock()
	s.running[taskID] = true
	s.canceled[taskID] = true
	s.mu.Unlock()

	s.clearRunState(taskID)

	s.mu.Lock()
	defer s.mu.Unlock()
	assert.False(t, s.running[taskID], "running flag should be cleared")
	assert.False(t, s.canceled[taskID], "canceled flag should be cleared")
}

func TestExecuteTask_SkipsNonWhitelistedChat(t *testing.T) {
	viper.Set("telegram.whitelist_chat_ids", "200")
	defer viper.Reset()
	s := setupTestSchedulerWithMockCLI(t)

	task := &model.ScheduledTask{
		ID: "exec-not-whitelisted", ChatID: 100, Prompt: "p",
		ScheduleType: model.ScheduleInterval, ScheduleValue: "1h",
		ContextMode: model.ContextGroup, Status: model.StatusActive, CreatedAt: time.Now(),
	}
	require.NoError(t, s.db.Create(task).Error)

	s.executeTask(task.ID)

	var count int64
	s.db.Model(&model.TaskRunLog{}).Where("task_id = ?", task.ID).Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestRecordResults_PausedDuringRunStaysPaused(t *testing.T) {
	s := setupTestSchedulerWithMockCLI(t)
	s.Start()
	defer s.Shutdown()

	task := &model.ScheduledTask{
		ID: "paused-mid-run", ChatID: 100, Prompt: "p",
		ScheduleType: model.ScheduleInterval, ScheduleValue: "1h",
		ContextMode: model.ContextGroup, Status: model.StatusActive, CreatedAt: time.Now(),
	}
	require.NoError(t, s.CreateTask(task))
	require.NoError(t, s.PauseTask(task.ID))

	require.NoError(t, s.recordResults(task, "reply", nil, time.Second))

	var stored model.ScheduledTask
	require.NoError(t, s.db.First(&stored, "id = ?", task.ID).Error)
	assert.Equal(t, model.StatusPaused, stored.Status)
	require.NoError(t, s.ResumeTask(task.ID))
}

func TestRecordResults_OnceSuccessCompletes(t *testing.T) {
	s := setupTestSchedulerWithMockCLI(t)
	s.Start()
	defer s.Shutdown()

	task := &model.ScheduledTask{
		ID: "once-success", ChatID: 100, Prompt: "p",
		ScheduleType:  model.ScheduleOnce,
		ScheduleValue: time.Now().Add(time.Hour).UTC().Format("2006-01-02T15:04:05"),
		ContextMode:   model.ContextIsolated, Status: model.StatusActive, CreatedAt: time.Now(),
	}
	require.NoError(t, s.db.Create(task).Error)

	require.NoError(t, s.recordResults(task, "reply", nil, time.Second))

	var stored model.ScheduledTask
	require.NoError(t, s.db.First(&stored, "id = ?", task.ID).Error)
	assert.Equal(t, model.StatusCompleted, stored.Status)
}

func TestCreateTask_PersistsNextRun(t *testing.T) {
	s := setupTestScheduler(t)
	s.Start()
	defer s.Shutdown()

	task := &model.ScheduledTask{
		ID: "persist-next", ChatID: 100, Prompt: "p",
		ScheduleType: model.ScheduleInterval, ScheduleValue: "2h",
		ContextMode: model.ContextGroup, Status: model.StatusActive, CreatedAt: time.Now(),
	}
	require.NoError(t, s.CreateTask(task))

	var stored model.ScheduledTask
	require.NoError(t, s.db.First(&stored, "id = ?", task.ID).Error)
	require.NotNil(t, stored.NextRun)
	assert.WithinDuration(t, time.Now().Add(2*time.Hour), *stored.NextRun, time.Minute)
}

func TestLoadTasks_IntervalKeepsPersistedNextRun(t *testing.T) {
	s := setupTestScheduler(t)

	next := time.Now().Add(30 * time.Minute)
	task := &model.ScheduledTask{
		ID: "interval-restore", ChatID: 100, Prompt: "p",
		ScheduleType: model.ScheduleInterval, ScheduleValue: "24h",
		ContextMode: model.ContextGroup, Status: model.StatusActive, CreatedAt: time.Now(), NextRun: &next,
	}
	require.NoError(t, s.db.Create(task).Error)

	s.LoadTasks()
	s.Start()
	defer s.Shutdown()

	got := s.resolveNextRun(task)
	require.NotNil(t, got)
	assert.WithinDuration(t, next, *got, time.Second)
}

func TestRestoredStart(t *testing.T) {
	future := time.Now().Add(time.Hour)
	past := time.Now().Add(-time.Hour)
	tests := []struct {
		name     string
		task     model.ScheduledTask
		wantOpts int
	}{
		{"interval with future next run", model.ScheduledTask{ScheduleType: model.ScheduleInterval, NextRun: &future}, 1},
		{"interval with missed next run", model.ScheduledTask{ScheduleType: model.ScheduleInterval, NextRun: &past}, 1},
		{"interval never run anchors at creation", model.ScheduledTask{ScheduleType: model.ScheduleInterval, CreatedAt: past}, 1},
		{"interval without next run or creation time", model.ScheduledTask{ScheduleType: model.ScheduleInterval}, 0},
		{"cron ignores next run", model.ScheduledTask{ScheduleType: model.ScheduleCron, NextRun: &future}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Len(t, restoredStart(&tt.task), tt.wantOpts)
		})
	}
}

func TestLoadTasks_NeverRunIntervalKeepsCadenceFromCreation(t *testing.T) {
	s := setupTestScheduler(t)

	created := time.Now().Add(-90 * time.Minute)
	task := &model.ScheduledTask{
		ID: "interval-legacy", ChatID: 100, Prompt: "p",
		ScheduleType: model.ScheduleInterval, ScheduleValue: "1h",
		ContextMode: model.ContextGroup, Status: model.StatusActive, CreatedAt: created,
	}
	require.NoError(t, s.db.Create(task).Error)

	s.LoadTasks()
	s.Start()
	defer s.Shutdown()

	got := s.resolveNextRun(task)
	require.NotNil(t, got)
	assert.WithinDuration(t, created.Add(2*time.Hour), *got, time.Second)
}
