# Plan: Unified Message Processing Pipeline

Currently, there are 3 different mechanisms for processing incoming messages and responses from Claude: telegram, schedule, and webhook. Essentially, these are just different message delivery paths, but right now their logic differs, especially when it comes to handling custom hooks such as send-file, webhook, and schedule. For example, if a message comes in via a webhook, Claude should still be able to create a schedule or another webhook. The same applies to schedule — it should be possible to create other schedules, webhooks, etc. So we need to refactor the code so that all 3 channels process messages in the same way, through a single codebase, without duplicating logic. In the future, there may be more incoming message channels. The outgoing channel is currently just one (Telegram), but there may be more in the future.
Three input channels (handler, scheduler, webhook) independently implement post-Claude processing with duplicated and inconsistent logic. Extract a shared `internal/pipeline/` package so all channels process Claude responses identically through a single `Process()` call.

## Validation Commands
- `CGO_ENABLED=1 go test ./...`
- `golangci-lint run ./...`
- `go build ./cmd/nclaw`

### Task 1: Create `internal/pipeline/` package
- [x] Create `internal/pipeline/pipeline.go` with `BlockExecutor` interface, `SendFunc` type, `Pipeline` struct, `New()` constructor
- [x] Implement `Process()`: execute blocks on `FullText` (gated on `claudeErr==nil`), strip from `Text`, append status, sendfile, send reply with HTML fallback
- [x] Own copies of schedule/webhook block regexes for stripping (avoids import cycle)
- [x] Create `internal/pipeline/pipeline_test.go` with mock `BlockExecutor`: success path, error path skips execution, nil webhook, status appending, empty text

### Task 2: Refactor handler to use pipeline
- [x] Replace `WebhookManager` and `SendDoc` fields with `Pipeline *pipeline.Pipeline`
- [x] Simplify `callClaude()`: keep only Claude invocation + `FormatTaskList`, remove all block logic
- [x] Simplify `processMessage()`: call `h.Pipeline.Process()`, remove manual block processing
- [x] Remove `appendStatus()`, `sendReply()`, `sendChunk()` (moved to pipeline)
- [x] Update `handler_test.go` for new struct fields

### Task 3: Refactor scheduler to use pipeline
- [x] Add `pipeline` field + `SetPipeline()` method to `Scheduler`
- [x] Remove `sendDoc` from `Scheduler` struct and `New()` constructor
- [x] Simplify `sendResult()`: delegate to `s.pipeline.Process()`
- [x] Remove `webhookBlockRe` from `commands.go`
- [x] Update scheduler tests for new constructor signature

### Task 4: Refactor webhook to use pipeline
- [x] Add `pipeline` field + `SetPipeline()` method to `Manager`
- [x] Simplify `processIncoming()`: delegate to `m.pipeline.Process()`
- [x] Remove `sendReply()`, `sendChunk()`, `send` field from `Manager`
- [x] Update `callClaude()` to return `(*claude.Result, error)`

### Task 5: Update wiring in `cmd/nclaw/main.go`
- [ ] Create single `SendFunc` (with `parseMode`) used by pipeline
- [ ] Construct `Pipeline` with scheduler, webhook (nil-safe), sendDoc, sendFunc
- [ ] Call `SetPipeline()` on scheduler and webhook before `Start()`
- [ ] Remove old `h.SendDoc`, `h.WebhookManager`, `newSendFunc()`
- [ ] Handle Go nil interface trap for webhook `BlockExecutor`

### Task 6: Finalize
- [ ] Update `CLAUDE.md` project structure and patterns
- [ ] Run `go test ./...` and `golangci-lint run ./...`
