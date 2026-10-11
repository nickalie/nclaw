package telegram

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// Prompt is the system prompt for formatting output as Telegram HTML.
const Prompt = `IMPORTANT: Your output will be displayed in Telegram.
Format all responses using Telegram HTML. Supported tags:
<b>bold</b>, <i>italic</i>, <u>underline</u>, <s>strikethrough</s>,
<code>inline code</code>, <pre>code block</pre>, <pre><code class="language-go">code with language</code></pre>,
<a href="URL">link</a>, <blockquote>quote</blockquote>, <tg-spoiler>spoiler</tg-spoiler>

Rules:
- Do NOT use Markdown syntax (no #headers, no **bold**, no backticks for code)
- Use ONLY the HTML tags listed above. No other HTML tags are supported.
- Escape &, < and > in regular text as &amp; &lt; &gt; (but not inside tags themselves)
- Do NOT use <p>, <br>, <div>, <h1>-<h6>, <ul>, <li>, <ol>, <table>, or any other HTML tags
- For lists, use plain text with bullet characters or numbers
- For section titles, use <b>bold text</b> on its own line
- Keep formatting minimal and clean`

// MaxMessageLen is the Telegram message size limit in characters.
const MaxMessageLen = 4096

// SplitMessage splits text into chunks of at most maxLen characters, breaking at newlines.
func SplitMessage(text string, maxLen int) []string {
	if len(text) <= maxLen {
		return []string{text}
	}

	var chunks []string
	for text != "" {
		if len(text) <= maxLen {
			chunks = append(chunks, text)
			break
		}
		cut := strings.LastIndex(text[:maxLen], "\n")
		if cut <= 0 {
			cut = runeBoundary(text, maxLen)
		}
		chunks = append(chunks, text[:cut])
		text = strings.TrimLeft(text[cut:], "\n")
	}
	return chunks
}

// runeBoundary moves n back to the start of a UTF-8 rune so a forced cut never splits a character.
func runeBoundary(text string, n int) int {
	for i := n; i > 0; i-- {
		if utf8.RuneStart(text[i]) {
			return i
		}
	}
	return n
}

// ChatDir returns the session directory for a given chat/thread under the base data directory.
func ChatDir(dataDir string, chatID int64, threadID int) string {
	dir := filepath.Join(dataDir, fmt.Sprintf("%d", chatID))
	if threadID != 0 {
		dir = filepath.Join(dir, fmt.Sprintf("%d", threadID))
	}
	return dir
}
