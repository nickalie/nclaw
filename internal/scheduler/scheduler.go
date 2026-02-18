package scheduler

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/nickalie/nclaw/internal/claude"
	"github.com/nickalie/nclaw/internal/db"
	"github.com/nickalie/nclaw/internal/model"
)

// SendFunc sends a text message to a Telegram chat/thread.
type SendFunc func(ctx context.Context, chatID int64, threadID int, text string) error

// Scheduler manages scheduled tasks using gocron and SQLite persistence.
type Scheduler struct {
	cron    gocron.Scheduler
	db      *gorm.DB
	send    SendFunc
	dataDir string
	jobs    map[string]uuid.UUID // taskID -> gocron job UUID
	mu      sync.Mutex
}

// New creates a new Scheduler.
func New(database *gorm.DB, send SendFunc, timezone, dataDir string) (*Scheduler, error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.Local
	}

	cron, err := gocron.NewScheduler(gocron.WithLocation(loc))
	if err != nil {
		return nil, fmt.Errorf("scheduler: create: %w", err)
	}

	return &Scheduler{
		cron:    cron,
		db:      database,
		send:    send,
		dataDir: dataDir,
		jobs:    make(map[string]uuid.UUID),
	}, nil
}

// Start begins the gocron scheduler.
func (s *Scheduler) Start() {
	s.cron.Start()
	log.Println("scheduler: started")
}

// Shutdown stops the gocron scheduler.
func (s *Scheduler) Shutdown() error {
	return s.cron.Shutdown()
}

// LoadTasks reads all active tasks from the database and registers them with gocron.
func (s *Scheduler) LoadTasks() {
	var tasks []model.ScheduledTask
	if err := s.db.Where("status = ?", model.StatusActive).Find(&tasks).Error; err != nil {
		log.Printf("scheduler: load tasks: %v", err)
		return
	}

	for i := range tasks {
		if err := s.addJob(&tasks[i]); err != nil {
			log.Printf("scheduler: load task %s: %v", tasks[i].ID, err)
		}
	}

	log.Printf("scheduler: loaded %d active tasks", len(tasks))
}

// CreateTask persists a task and registers it with gocron.
func (s *Scheduler) CreateTask(task *model.ScheduledTask) error {
	if err := db.CreateTask(s.db, task); err != nil {
		return fmt.Errorf("scheduler: create task: %w", err)
	}
	if err := s.addJob(task); err != nil {
		return fmt.Errorf("scheduler: add job: %w", err)
	}
	log.Printf("scheduler: created task %s (%s: %s) prompt=%q", task.ID, task.ScheduleType, task.ScheduleValue, truncate(task.Prompt, 60))
	return nil
}

// PauseTask pauses a task by removing it from gocron and updating status.
func (s *Scheduler) PauseTask(id string) error {
	s.removeJob(id)
	log.Printf("scheduler: paused task %s", id)
	return db.UpdateTaskStatus(s.db, id, model.StatusPaused)
}

// ResumeTask resumes a paused task.
func (s *Scheduler) ResumeTask(id string) error {
	task, err := db.GetTask(s.db, id)
	if err != nil {
		return fmt.Errorf("scheduler: get task: %w", err)
	}

	if err := db.UpdateTaskStatus(s.db, id, model.StatusActive); err != nil {
		return err
	}
	task.Status = model.StatusActive

	log.Printf("scheduler: resumed task %s", id)
	return s.addJob(task)
}

// CancelTask deletes a task entirely.
func (s *Scheduler) CancelTask(id string) error {
	s.removeJob(id)
	log.Printf("scheduler: canceled task %s", id)
	return db.DeleteTask(s.db, id)
}

// addJob creates a gocron job for the given task.
func (s *Scheduler) addJob(task *model.ScheduledTask) error {
	def, err := s.jobDefinition(task)
	if err != nil {
		return err
	}

	taskID := task.ID
	job, err := s.cron.NewJob(
		def,
		gocron.NewTask(s.executeTask, taskID),
		gocron.WithName(taskID),
	)
	if err != nil {
		return fmt.Errorf("new job: %w", err)
	}

	s.mu.Lock()
	s.jobs[taskID] = job.ID()
	s.mu.Unlock()

	return nil
}

// removeJob removes a gocron job for the given task ID.
func (s *Scheduler) removeJob(taskID string) {
	s.mu.Lock()
	jobID, ok := s.jobs[taskID]
	if ok {
		delete(s.jobs, taskID)
	}
	s.mu.Unlock()

	if ok {
		_ = s.cron.RemoveJob(jobID)
	}
}

func (s *Scheduler) jobDefinition(task *model.ScheduledTask) (gocron.JobDefinition, error) {
	switch task.ScheduleType {
	case model.ScheduleCron:
		return gocron.CronJob(task.ScheduleValue, false), nil
	case model.ScheduleInterval:
		d, err := time.ParseDuration(task.ScheduleValue)
		if err != nil {
			return nil, fmt.Errorf("parse interval %q: %w", task.ScheduleValue, err)
		}
		return gocron.DurationJob(d), nil
	case model.ScheduleOnce:
		t, err := time.ParseInLocation("2006-01-02T15:04:05", task.ScheduleValue, time.Local)
		if err != nil {
			return nil, fmt.Errorf("parse once time %q: %w", task.ScheduleValue, err)
		}
		return gocron.OneTimeJob(gocron.OneTimeJobStartDateTime(t)), nil
	default:
		return nil, fmt.Errorf("unknown schedule type %q", task.ScheduleType)
	}
}

func (s *Scheduler) executeTask(taskID string) {
	task, err := db.GetTask(s.db, taskID)
	if err != nil {
		log.Printf("scheduler: execute: task %s not found: %v", taskID, err)
		return
	}
	if task.Status != model.StatusActive {
		log.Printf("scheduler: skipping task %s (status=%s)", taskID, task.Status)
		return
	}

	log.Printf("scheduler: executing task %s (%s: %s) prompt=%q",
		taskID, task.ScheduleType, task.ScheduleValue, truncate(task.Prompt, 60))

	start := time.Now()

	dir := filepath.Join(s.dataDir, fmt.Sprintf("%d", task.ThreadID))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Printf("scheduler: mkdir %s: %v", dir, err)
	}

	prompt := "[SCHEDULED TASK - Running automatically, not in response to a user message]\n\n" + task.Prompt
	reply, runErr := s.invokeClaudeForTask(task, dir, prompt)
	duration := time.Since(start)

	if runErr != nil {
		log.Printf("scheduler: task %s failed after %s: %v", taskID, duration, runErr)
	} else {
		log.Printf("scheduler: task %s completed in %s, reply_len=%d", taskID, duration, len(reply))
	}

	s.logAndUpdate(task, reply, runErr, duration)
	s.sendResult(task, reply, runErr)
}

func (s *Scheduler) invokeClaudeForTask(task *model.ScheduledTask, dir, prompt string) (string, error) {
	log.Printf("scheduler: invoking claude for task %s in dir=%s context=%s", task.ID, dir, task.ContextMode)
	c := claude.New().Dir(dir).SkipPermissions()
	if task.ContextMode == model.ContextGroup {
		return c.Continue(prompt)
	}
	return c.Ask(prompt)
}

func (s *Scheduler) logAndUpdate(task *model.ScheduledTask, reply string, runErr error, duration time.Duration) {
	s.logRun(task.ID, reply, runErr, duration)
	s.updateAfterRun(task, reply, runErr)
}

func (s *Scheduler) logRun(taskID, reply string, runErr error, duration time.Duration) {
	runLog := &model.TaskRunLog{
		TaskID:     taskID,
		RunAt:      time.Now(),
		DurationMs: duration.Milliseconds(),
		Status:     "success",
	}

	if runErr != nil {
		errStr := runErr.Error()
		runLog.Status = "error"
		runLog.Error = &errStr
	} else {
		runLog.Result = &reply
	}

	if err := db.LogRun(s.db, runLog); err != nil {
		log.Printf("scheduler: log run: %v", err)
	}
}

func (s *Scheduler) updateAfterRun(task *model.ScheduledTask, reply string, runErr error) {
	nextRun := s.resolveNextRun(task)

	resultSummary := truncate(reply, 200)
	if runErr != nil {
		resultSummary = "Error: " + runErr.Error()
	}

	if err := db.UpdateTaskAfterRun(s.db, task.ID, nextRun, resultSummary); err != nil {
		log.Printf("scheduler: update after run: %v", err)
	}

	if runErr != nil && task.ScheduleType == model.ScheduleOnce {
		_ = db.UpdateTaskStatus(s.db, task.ID, model.StatusFailed)
	}

	if task.ScheduleType == model.ScheduleOnce {
		s.removeJob(task.ID)
	}
}

func (s *Scheduler) resolveNextRun(task *model.ScheduledTask) *time.Time {
	if task.ScheduleType == model.ScheduleOnce {
		return nil
	}

	s.mu.Lock()
	jobID, ok := s.jobs[task.ID]
	s.mu.Unlock()

	if !ok {
		return nil
	}
	return s.getNextRun(jobID)
}

func (s *Scheduler) sendResult(task *model.ScheduledTask, reply string, runErr error) {
	text := reply
	if runErr != nil {
		text = "Scheduled task error: " + runErr.Error()
	}

	// Process any schedule commands embedded in the reply (e.g. cancel self).
	text = s.ProcessReply(text, task.ChatID, task.ThreadID)

	if text == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := s.send(ctx, task.ChatID, task.ThreadID, text); err != nil {
		log.Printf("scheduler: send result for task %s: %v", task.ID, err)
	} else {
		log.Printf("scheduler: sent result for task %s to chat=%d thread=%d", task.ID, task.ChatID, task.ThreadID)
	}
}

func (s *Scheduler) getNextRun(jobID uuid.UUID) *time.Time {
	for _, j := range s.cron.Jobs() {
		if j.ID() == jobID {
			t, err := j.NextRun()
			if err == nil {
				return &t
			}
		}
	}
	return nil
}

// FormatTaskList returns a system prompt section listing current tasks for a chat/thread.
func (s *Scheduler) FormatTaskList(chatID int64, threadID int) string {
	tasks, err := db.ListTasksByChat(s.db, chatID, threadID)
	if err != nil {
		log.Printf("scheduler: list tasks: %v", err)
		return "Current scheduled tasks: none"
	}
	if len(tasks) == 0 {
		return "Current scheduled tasks: none"
	}

	var b strings.Builder
	b.WriteString("Current scheduled tasks:\n")
	for i := range tasks {
		next := "N/A"
		if tasks[i].NextRun != nil {
			next = tasks[i].NextRun.Format("2006-01-02 15:04:05")
		}
		prompt := truncate(tasks[i].Prompt, 80)
		fmt.Fprintf(&b, "- [%s] %s (%s: %s) status=%s next=%s\n",
			tasks[i].ID, prompt, tasks[i].ScheduleType, tasks[i].ScheduleValue, tasks[i].Status, next)
	}
	return b.String()
}

func truncate(str string, maxLen int) string {
	if len(str) <= maxLen {
		return str
	}
	return str[:maxLen]
}
