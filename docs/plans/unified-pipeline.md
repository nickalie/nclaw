# Plan: Unified Message Processing Pipeline

Currently, there are 3 different mechanisms for processing incoming messages and responses from Claude: telegram, schedule, and webhook. Essentially, these are just different message delivery paths, but right now their logic differs, especially when it comes to handling custom hooks such as send-file, webhook, and schedule. For example, if a message comes in via a webhook, Claude should still be able to create a schedule or another webhook. The same applies to schedule — it should be possible to create other schedules, webhooks, etc. So we need to refactor the code so that all 3 channels process messages in the same way, through a single codebase, without duplicating logic. In the future, there may be more incoming message channels. The outgoing channel is currently just one (Telegram), but there may be more in the future.
Three input channels (handler, scheduler, webhook) independently implement post-Claude processing with duplicated and inconsistent logic. Extract a shared `internal/pipeline/` package so all channels process Claude responses identically through a single `Process()` call.

## Validation Commands
- `CGO_ENABLED=1 go test ./...`
- `golangci-lint run ./...`
- `go build ./cmd/nclaw`

### Task 1: Create `internal/pipeline/` package
- [ ] Create `internal/pipeline/pipeline.go` with `BlockExecutor` interface, `SendFunc` type, `Pipeline` struct, `New()` constructor
- [ ] Implement `Process()`: execute blocks on `FullText` (gated on `claudeErr==nil`), strip from `Text`, append status, sendfile, send reply with HTML fallback
- [ ] Own copies of schedule/webhook block regexes for stripping (avoids import cycle)
- [ ] Create `internal/pipeline/pipeline_test.go` with mock `BlockExecutor`: success path, error path skips execution, nil webhook, status appending, empty text

### Task 2: Refactor handler to use pipeline
- [ ] Replace `WebhookManager` and `SendDoc` fields with `Pipeline *pipeline.Pipeline`
- [ ] Simplify `callClaude()`: keep only Claude invocation + `FormatTaskList`, remove all block logic
- [ ] Simplify `processMessage()`: call `h.Pipeline.Process()`, remove manual block processing
- [ ] Remove `appendStatus()`, `sendReply()`, `sendChunk()` (moved to pipeline)
- [ ] Update `handler_test.go` for new struct fields

### Task 3: Refactor scheduler to use pipeline
- [ ] Add `pipeline` field + `SetPipeline()` method to `Scheduler`
- [ ] Remove `sendDoc` from `Scheduler` struct and `New()` constructor
- [ ] Simplify `sendResult()`: delegate to `s.pipeline.Process()`
- [ ] Remove `webhookBlockRe` from `commands.go`
- [ ] Update scheduler tests for new constructor signature

### Task 4: Refactor webhook to use pipeline
- [ ] Add `pipeline` field + `SetPipeline()` method to `Manager`
- [ ] Simplify `processIncoming()`: delegate to `m.pipeline.Process()`
- [ ] Remove `sendReply()`, `sendChunk()`, `send` field from `Manager`
- [ ] Update `callClaude()` to return `(*claude.Result, error)`

### Task 5: Update wiring in `cmd/nclaw/main.go`
- [ ] Create single `SendFunc` (with `parseMode`) used by pipeline
- [ ] Construct `Pipeline` with scheduler, webhook (nil-safe), sendDoc, sendFunc
- [ ] Call `SetPipeline()` on scheduler and webhook before `Start()`
- [ ] Remove old `h.SendDoc`, `h.WebhookManager`, `newSendFunc()`
- [ ] Handle Go nil interface trap for webhook `BlockExecutor`

### Task 6: Finalize
- [ ] Update `CLAUDE.md` project structure and patterns
- [ ] Run `go test ./...` and `golangci-lint run ./...`
