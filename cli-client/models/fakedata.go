package models

import "time"

// FakeData contains demo messages for the chat.
// Color values are tview color tags (e.g. "[magenta]").
var FakeData = []*Message{
	{
		ID:        "1",
		Username:  "root",
		Content:   "Welcome to the global persistent stream. High-speed TUI active.",
		Timestamp: time.Date(2024, 1, 15, 14, 1, 0, 0, time.UTC),
		IsSystem:  false,
		Color:     "[magenta]",
	},
	{
		ID:        "2",
		Username:  "cyber_punk",
		Content:   "Anyone running the new Go-lang binaries on Termux?",
		Timestamp: time.Date(2024, 1, 15, 14, 2, 0, 0, time.UTC),
		IsSystem:  false,
		Color:     "[green]",
	},
	{
		ID:        "3",
		Username:  "SYSTEM",
		Content:   "End-to-end encryption active for global relay.",
		Timestamp: time.Date(2024, 1, 15, 14, 5, 0, 0, time.UTC),
		IsSystem:  true,
		Color:     "[yellow]",
	},
	{
		ID:        "4",
		Username:  "script_kiddie",
		Content:   "What is the #main topic today?",
		Timestamp: time.Date(2024, 1, 15, 14, 7, 0, 0, time.UTC),
		IsSystem:  false,
		Color:     "[yellow]",
	},
	{
		ID:        "5",
		Username:  "gopher_dev",
		Content:   "Optimizing the TUI for iOS Termux users. Keep it minimal.",
		Timestamp: time.Date(2024, 1, 15, 14, 8, 0, 0, time.UTC),
		IsSystem:  false,
		Color:     "[magenta]",
	},
	{
		ID:        "6",
		Username:  "anon_x",
		Content:   "The latency is impressive for a global node.",
		Timestamp: time.Date(2024, 1, 15, 14, 10, 0, 0, time.UTC),
		IsSystem:  false,
		Color:     "[green]",
	},
	{
		ID:        "7",
		Username:  "root",
		Content:   "Check /help for new global stream commands.",
		Timestamp: time.Date(2024, 1, 15, 14, 12, 0, 0, time.UTC),
		IsSystem:  false,
		Color:     "[magenta]",
	},
}

// GetFakeUsers returns fake online users with tview color tags.
func GetFakeUsers() map[string]*User {
	return map[string]*User{
		"root":          {Username: "root", Color: "[magenta]", IsOnline: true},
		"cyber_punk":    {Username: "cyber_punk", Color: "[green]", IsOnline: true},
		"script_kiddie": {Username: "script_kiddie", Color: "[yellow]", IsOnline: true},
		"gopher_dev":    {Username: "gopher_dev", Color: "[magenta]", IsOnline: true},
		"anon_x":        {Username: "anon_x", Color: "[green]", IsOnline: true},
	}
}
