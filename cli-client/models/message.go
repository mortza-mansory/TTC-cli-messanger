package models

import "time"

// Message represents a chat message.
// Color is a tview color tag string e.g. "[green]" or "[#ff00ff]".
type Message struct {
	ID        string
	Username  string
	Content   string
	Timestamp time.Time
	IsSystem  bool
	Color     string // tview color tag — used for both username label and content text
}

// NewMessage creates a new outgoing message with the default hash-based color.
// The controller should override Color via AppState.GetUserColorTag if the user
// has set a custom color.
func NewMessage(username, content string) *Message {
	return &Message{
		ID:        generateMessageID(),
		Username:  username,
		Content:   content,
		Timestamp: time.Now(),
		IsSystem:  false,
		Color:     GetUsernameColor(username), // tview tag e.g. "[magenta]"
	}
}

// NewSystemMessage creates a system notification message.
func NewSystemMessage(content string) *Message {
	return &Message{
		ID:        generateMessageID(),
		Username:  "SYSTEM",
		Content:   content,
		Timestamp: time.Now(),
		IsSystem:  true,
		Color:     "[yellow]",
	}
}

// FormatTime returns the formatted timestamp for display.
func (m *Message) FormatTime() string {
	return m.Timestamp.Format("15:04")
}

func generateMessageID() string {
	return time.Now().Format("20060102150405")
}
