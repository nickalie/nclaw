---
name: schedule
description: "Schedule recurring or one-time tasks. Use when user asks to schedule something, set reminders, repeat actions on interval, set up recurring jobs, or manage existing scheduled tasks (list, pause, resume, cancel). Triggers on words like 'remind', 'schedule', 'every X minutes/hours', 'at 9am', 'recurring', 'repeat', 'timer', 'alarm'."
---

# Task Scheduling

You can schedule tasks by embedding scheduling commands in your response. The system parses these commands and executes them automatically.

## Output Format

Wrap each scheduling command in a fenced code block with language `nclaw:schedule`:

````
```nclaw:schedule
{"action":"create","prompt":"...","type":"...","value":"...","context":"group"}
```
````

## Actions

### Create a Task

Fields:
- `action`: `"create"` (required)
- `prompt`: What to do when the task fires (required). Be specific and self-contained.
- `type`: Schedule type (required). Choose carefully:
  - `"cron"` — Standard 5-field cron expression (minute hour day month weekday). For recurring at specific times.
  - `"interval"` — Go duration string (e.g., `"5m"`, `"1h30m"`, `"24h"`). For REPEATING every N time. Only use when the user explicitly wants repetition ("every hour", "each 5 minutes").
  - `"once"` — Local ISO 8601 timestamp WITHOUT "Z" suffix. For ONE-TIME reminders/actions. Use this for "remind me in X minutes", "do X at 3pm", "in 1 hour do Y". Calculate the target timestamp from current time.
- `value`: The schedule expression (required)
- `context`: `"group"` (default) or `"isolated"` (optional)

Context modes:
- `"group"`: Task runs in the conversation's session (has chat history). Use for tasks needing context.
- `"isolated"`: Task runs in a fresh session. Use for self-contained tasks. Include all needed context in the prompt.

### Pause, Resume, Cancel

```json
{"action":"pause","task_id":"task-..."}
{"action":"resume","task_id":"task-..."}
{"action":"cancel","task_id":"task-..."}
```

## Schedule Value Examples

**Cron:**
- `"0 9 * * *"` — daily at 9:00 AM local time
- `"*/30 * * * *"` — every 30 minutes
- `"0 9 * * 1-5"` — weekdays at 9 AM
- `"0 0 1 * *"` — first day of every month at midnight

**Interval:**
- `"5m"` — every 5 minutes
- `"1h"` — every hour
- `"24h"` — every 24 hours
- `"30s"` — every 30 seconds

**Once:**
- `"2026-02-18T15:30:00"` — at 3:30 PM local time on Feb 18, 2026

## Current Tasks

The system provides a list of currently scheduled tasks in your context. Use this information to:
- Answer questions about existing schedules
- Find task IDs for pause/resume/cancel operations
- Avoid creating duplicate tasks

## Rules

1. Always confirm to the user what was scheduled in your response text.
2. One command per code block. Use multiple code blocks for multiple actions.
3. For "once" tasks, never add a "Z" suffix — all times are local.
4. If the user's request is ambiguous about timing, ask for clarification.
5. When canceling or pausing, reference the task ID from the current tasks list.
6. Include the scheduling code block naturally within your response — the system will strip it before showing the response to the user.

## Examples

User: "Remind me to drink water every hour"
Response: Sure, I'll remind you every hour to drink water!
```nclaw:schedule
{"action":"create","prompt":"Send a reminder: Time to drink water! Stay hydrated.","type":"interval","value":"1h","context":"isolated"}
```

User: "Every weekday at 9am tell me good morning"
Response: Done! You'll get a good morning message every weekday at 9 AM.
```nclaw:schedule
{"action":"create","prompt":"Send a cheerful good morning greeting to start the day.","type":"cron","value":"0 9 * * 1-5","context":"isolated"}
```

User: "Remind me in 5 minutes to check the oven"
Response: Got it, I'll remind you in 5 minutes!
```nclaw:schedule
{"action":"create","prompt":"Reminder: Check the oven!","type":"once","value":"2026-02-18T12:55:00","context":"isolated"}
```
Note: "in X minutes" = once (calculate timestamp), NOT interval.

User: "Remind me at 3pm today to call the dentist"
Response: I'll remind you at 3 PM today to call the dentist.
```nclaw:schedule
{"action":"create","prompt":"Reminder: Call the dentist!","type":"once","value":"2026-02-18T15:00:00","context":"isolated"}
```

User: "Cancel all my reminders"
Response: I've cancelled all your scheduled tasks.
```nclaw:schedule
{"action":"cancel","task_id":"task-123-abc"}
```
```nclaw:schedule
{"action":"cancel","task_id":"task-456-def"}
```
