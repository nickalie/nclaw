package scheduler

import (
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/nickalie/nclaw/internal/model"
)

var scheduleBlockRe = regexp.MustCompile("(?s)```nclaw:schedule\n(.*?)\n```")

// StripBlocks removes nclaw:schedule code blocks from text without executing them.
func StripBlocks(text string) string {
	return strings.TrimSpace(scheduleBlockRe.ReplaceAllString(text, ""))
}

type scheduleCommand struct {
	Action  string `json:"action"`
	Prompt  string `json:"prompt"`
	Type    string `json:"type"`
	Value   string `json:"value"`
	Context string `json:"context"`
	TaskID  string `json:"task_id"`
}

// ExecuteBlocks extracts nclaw:schedule code blocks from text, executes them,
// and returns any status messages (errors). Does not modify the input text.
func (s *Scheduler) ExecuteBlocks(text string, chatID int64, threadID int) string {
	matches := scheduleBlockRe.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return ""
	}

	var errs []string

	for _, match := range matches {
		if err := s.executeCommand(match[1], chatID, threadID); err != nil {
			log.Printf("scheduler: command error: %v", err)
			errs = append(errs, err.Error())
		}
	}

	if len(errs) > 0 {
		return "[Schedule error: " + strings.Join(errs, "; ") + "]"
	}
	return ""
}

func (s *Scheduler) executeCommand(jsonStr string, chatID int64, threadID int) error {
	var cmd scheduleCommand
	if err := json.Unmarshal([]byte(jsonStr), &cmd); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	log.Printf("scheduler: processing command action=%s task_id=%s", cmd.Action, cmd.TaskID)

	switch cmd.Action {
	case "create":
		return s.createTaskFromCommand(&cmd, chatID, threadID)
	case "pause":
		return s.PauseTask(cmd.TaskID)
	case "resume":
		return s.ResumeTask(cmd.TaskID)
	case "cancel":
		return s.CancelTask(cmd.TaskID)
	default:
		return fmt.Errorf("unknown action %q", cmd.Action)
	}
}

func (s *Scheduler) createTaskFromCommand(cmd *scheduleCommand, chatID int64, threadID int) error {
	if cmd.Prompt == "" || cmd.Type == "" || cmd.Value == "" {
		return fmt.Errorf("create requires prompt, type, and value")
	}

	contextMode := cmd.Context
	if contextMode == "" {
		contextMode = model.ContextGroup
	}

	var nextRun *time.Time
	if cmd.Type == model.ScheduleOnce {
		t, err := time.ParseInLocation("2006-01-02T15:04:05", cmd.Value, s.loc)
		if err != nil {
			return fmt.Errorf("invalid once time %q: %w", cmd.Value, err)
		}
		nextRun = &t
	}

	task := &model.ScheduledTask{
		ID:            model.GenerateTaskID(),
		ChatID:        chatID,
		ThreadID:      threadID,
		Prompt:        cmd.Prompt,
		ScheduleType:  cmd.Type,
		ScheduleValue: cmd.Value,
		ContextMode:   contextMode,
		Status:        model.StatusActive,
		NextRun:       nextRun,
		CreatedAt:     time.Now(),
	}

	return s.CreateTask(task)
}
