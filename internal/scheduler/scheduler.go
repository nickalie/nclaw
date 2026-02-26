package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/nickalie/nclaw/internal/cli"
	"github.com/nickalie/nclaw/internal/db"
	"github.com/nickalie/nclaw/internal/model"
	"github.com/nickalie/nclaw/internal/pipeline"
	"github.com/nickalie/nclaw/internal/telegram"
)

// Scheduler manages scheduled tasks using gocron and SQLite persistence.
type Scheduler struct {
	cron       gocron.Scheduler
	db         *gorm.DB
	provider   cli.Provider
	pipeline   *pipeline.Pipeline
	dataDir    string
	loc        *time.Location
	chatLocker *telegram.ChatLocker
	jobs       map[string]uuid.UUID // taskID -> gocron job UUID
	running    map[string]bool      // tasks currently executing
	canceled   map[string]bool      // tasks canceled during execution
	mu         sync.Mutex
}

// New creates a new Scheduler.
func New(
	database *gorm.DB, provider cli.Provider,
	timezone, dataDir string, chatLocker *telegram.ChatLocker,
) (*Scheduler, error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		log.Printf("scheduler: invalid timezone %q, falling back to local: %v", timezone, err)
		loc = time.Local
	}

	cron, err := gocron.NewScheduler(gocron.WithLocation(loc))
	if err != nil {
		return nil, fmt.Errorf("scheduler: create: %w", err)
	}

	return &Scheduler{
		cron:       cron,
		db:         database,
		provider:   provider,
		dataDir:    dataDir,
		loc:        loc,
		chatLocker: chatLocker,
		jobs:       make(map[string]uuid.UUID),
		running:    make(map[string]bool),
		canceled:   make(map[string]bool),
	}, nil
}

// SetPipeline sets the pipeline for post-Claude response processing.
func (s *Scheduler) SetPipeline(p *pipeline.Pipeline) {
	s.pipeline = p
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
		if delErr := db.DeleteTask(s.db, task.ID); delErr != nil {
			log.Printf("scheduler: rollback failed for task %s: %v", task.ID, delErr)
		}
		return fmt.Errorf("scheduler: add job: %w", err)
	}
	log.Printf("scheduler: created task %s (%s: %s) prompt=%q", task.ID, task.ScheduleType, task.ScheduleValue, truncate(task.Prompt, 60))
	return nil
}

// PauseTask pauses a task by removing it from gocron and updating status.
func (s *Scheduler) PauseTask(id string) error {
	task, err := db.GetTask(s.db, id)
	if err != nil {
		return fmt.Errorf("scheduler: get task: %w", err)
	}
	if task.Status != model.StatusActive {
		return fmt.Errorf("scheduler: task %s is %s, not active", id, task.Status)
	}

	if err := db.UpdateTaskStatus(s.db, id, model.StatusPaused); err != nil {
		return err
	}
	s.removeJob(id)
	log.Printf("scheduler: paused task %s", id)
	return nil
}

// ResumeTask resumes a paused task.
func (s *Scheduler) ResumeTask(id string) error {
	task, err := db.GetTask(s.db, id)
	if err != nil {
		return fmt.Errorf("scheduler: get task: %w", err)
	}
	if task.Status != model.StatusPaused {
		return fmt.Errorf("scheduler: task %s is %s, not paused", id, task.Status)
	}

	if err := db.UpdateTaskStatus(s.db, id, model.StatusActive); err != nil {
		return err
	}
	task.Status = model.StatusActive

	if err := s.addJob(task); err != nil {
		if rbErr := db.UpdateTaskStatus(s.db, id, model.StatusPaused); rbErr != nil {
			log.Printf("scheduler: rollback failed for task %s: %v", id, rbErr)
		}
		return fmt.Errorf("scheduler: resume add job: %w", err)
	}
	log.Printf("scheduler: resumed task %s", id)
	return nil
}

// CancelTask deletes a task entirely.
func (s *Scheduler) CancelTask(id string) error {
	s.mu.Lock()
	if s.running[id] {
		s.canceled[id] = true
	}
	s.mu.Unlock()

	if err := db.DeleteTask(s.db, id); err != nil {
		s.mu.Lock()
		delete(s.canceled, id)
		s.mu.Unlock()
		return fmt.Errorf("scheduler: delete task: %w", err)
	}
	s.removeJob(id)
	log.Printf("scheduler: canceled task %s", id)
	return nil
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
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
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
		t, err := time.ParseInLocation("2006-01-02T15:04:05", task.ScheduleValue, s.loc)
		if err != nil {
			return nil, fmt.Errorf("parse once time %q: %w", task.ScheduleValue, err)
		}
		return gocron.OneTimeJob(gocron.OneTimeJobStartDateTime(t)), nil
	default:
		return nil, fmt.Errorf("unknown schedule type %q", task.ScheduleType)
	}
}

func (s *Scheduler) executeTask(taskID string) {
	s.mu.Lock()
	s.running[taskID] = true
	s.mu.Unlock()
	defer s.clearRunState(taskID)

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

	dir := telegram.ChatDir(s.dataDir, task.ChatID, task.ThreadID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Printf("scheduler: mkdir %s: %v", dir, err)
	}

	prompt := "[SCHEDULED TASK - Running automatically, not in response to a user message]\n\n" + task.Prompt
	result, runErr := s.invokeCLI(task, dir, prompt)
	duration := time.Since(start)

	if result == nil {
		result = &cli.Result{}
	}

	if runErr != nil {
		log.Printf("scheduler: task %s failed after %s: %v", taskID, duration, runErr)
	} else {
		log.Printf("scheduler: task %s completed in %s, reply_len=%d", taskID, duration, len(result.Text))
	}

	s.finalizeAndSend(task, result, runErr, duration)
}

// finalizeAndSend records run results and sends the reply, unless the task was canceled.
func (s *Scheduler) finalizeAndSend(task *model.ScheduledTask, result *cli.Result, runErr error, duration time.Duration) {
	// Atomically verify task still exists and record results.
	err := s.recordResults(task, result.Text, runErr, duration)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		log.Printf("scheduler: task %s was deleted during execution, skipping send", task.ID)
		return
	}
	if err != nil {
		log.Printf("scheduler: task %s record results failed: %v", task.ID, err)
	}

	if task.ScheduleType == model.ScheduleOnce {
		s.removeJob(task.ID)
	}

	// Atomically check canceled and exit "running" state so CancelTask
	// can no longer flag this execution after we decide to send.
	s.mu.Lock()
	wasCanceled := s.canceled[task.ID]
	delete(s.canceled, task.ID)
	delete(s.running, task.ID)
	s.mu.Unlock()

	if wasCanceled {
		log.Printf("scheduler: task %s was canceled after execution, skipping send", task.ID)
		return
	}

	s.sendResult(task, result, runErr)
}

// clearRunState removes the task from both the running and canceled sets.
func (s *Scheduler) clearRunState(taskID string) {
	s.mu.Lock()
	delete(s.running, taskID)
	delete(s.canceled, taskID)
	s.mu.Unlock()
}

func (s *Scheduler) invokeCLI(task *model.ScheduledTask, dir, prompt string) (*cli.Result, error) {
	if err := s.provider.PreInvoke(); err != nil {
		log.Printf("scheduler: pre-invoke warning: %v", err)
	}

	unlock := s.chatLocker.Lock(task.ChatID, task.ThreadID)
	defer unlock()

	log.Printf("scheduler: invoking %s for task %s in dir=%s context=%s", s.provider.Name(), task.ID, dir, task.ContextMode)
	c := s.provider.NewClient().Dir(dir).SkipPermissions()
	if task.ContextMode == model.ContextGroup {
		return c.Continue(prompt)
	}
	return c.Ask(prompt)
}

// recordResults atomically verifies the task still exists and records the run outcome.
// Returns gorm.ErrRecordNotFound if the task was deleted (e.g. canceled during execution).
func (s *Scheduler) recordResults(task *model.ScheduledTask, reply string, runErr error, duration time.Duration) error {
	nextRun := s.resolveNextRun(task)
	return s.db.Transaction(func(tx *gorm.DB) error {
		if _, err := db.GetTask(tx, task.ID); err != nil {
			return err
		}
		if err := s.logRunTx(tx, task.ID, reply, runErr, duration); err != nil {
			return err
		}
		return s.updateAfterRunTx(tx, task, nextRun, reply, runErr)
	})
}

func (s *Scheduler) logRunTx(tx *gorm.DB, taskID, reply string, runErr error, duration time.Duration) error {
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
	return db.LogRun(tx, runLog)
}

func (s *Scheduler) updateAfterRunTx(
	tx *gorm.DB, task *model.ScheduledTask, nextRun *time.Time, reply string, runErr error,
) error {
	resultSummary := truncate(reply, 200)
	if runErr != nil {
		resultSummary = "Error: " + runErr.Error()
	}
	if err := db.UpdateTaskAfterRun(tx, task.ID, nextRun, resultSummary); err != nil {
		return err
	}
	if runErr != nil && task.ScheduleType == model.ScheduleOnce {
		return db.UpdateTaskStatus(tx, task.ID, model.StatusFailed)
	}
	return nil
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

func (s *Scheduler) sendResult(task *model.ScheduledTask, result *cli.Result, runErr error) {
	if s.pipeline == nil {
		log.Printf("scheduler: pipeline not ready, dropping result for task %s", task.ID)
		return
	}

	if runErr != nil {
		result = &cli.Result{
			Text:     "Scheduled task error: " + runErr.Error(),
			FullText: "Scheduled task error: " + runErr.Error(),
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dir := telegram.ChatDir(s.dataDir, task.ChatID, task.ThreadID)
	s.pipeline.Process(ctx, result, runErr, task.ChatID, task.ThreadID, dir)
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
	runes := []rune(str)
	if len(runes) <= maxLen {
		return str
	}
	return string(runes[:maxLen])
}
