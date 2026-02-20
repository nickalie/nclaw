package telegram

import "sync"

// ChatLocker provides per-chat mutex serialization.
// It ensures that only one Claude CLI call runs per chat/thread at a time.
type ChatLocker struct {
	mu sync.Map // map[chatKey]*sync.Mutex
}

type chatKey struct {
	chatID   int64
	threadID int
}

// NewChatLocker creates a new ChatLocker.
func NewChatLocker() *ChatLocker {
	return &ChatLocker{}
}

// Lock acquires the mutex for the given chat/thread and returns the unlock function.
func (cl *ChatLocker) Lock(chatID int64, threadID int) func() {
	key := chatKey{chatID: chatID, threadID: threadID}
	val, _ := cl.mu.LoadOrStore(key, &sync.Mutex{})
	mu := val.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}
