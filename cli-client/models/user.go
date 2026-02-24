package models

import (
	"strings"
	"time"
)

// User represents a chat user
type User struct {
	Username string
	Color    string // tview color tag e.g. "[magenta]"
	IsOnline bool
	LastSeen time.Time
}

// NewUser creates a new user with default values
func NewUser(username string) *User {
	return &User{
		Username: username,
		Color:    GetUsernameColor(username),
		IsOnline: true,
		LastSeen: time.Now(),
	}
}

// GetUsernameColor returns a deterministic tview color tag based on username hash.
// Returns tags like "[magenta]", "[green]", etc.
func GetUsernameColor(username string) string {
	tags := []string{
		"[magenta]",
		"[green]",
		"[cyan]",
		"[yellow]",
		"[red]",
		"[blue]",
	}
	hash := 0
	for _, c := range username {
		hash += int(c)
	}
	return tags[hash%len(tags)]
}

// ParseColorToTag converts a color value from an incoming JSON message into a
// tview-compatible color tag string.
//
// Supported input formats:
//   - "#rrggbb"  → "[#rrggbb]"   (6-digit hex, 24-bit)
//   - "#rgb"     → "[#rrggbb]"   (3-digit shorthand, expanded)
//   - "#rgba" / "#rrggbbaa" → alpha stripped, treated as RGB
//   - "green"    → "[green]"     (named tview/tcell color)
//   - "[green]"  → "[green]"     (already a tview tag, pass through)
//   - ""         → "[white]"     (fallback)
func ParseColorToTag(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "[white]"
	}
	// Already a tview tag
	if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
		return s
	}
	// Hex color
	if strings.HasPrefix(s, "#") {
		hex := strings.ToLower(s[1:])
		switch len(hex) {
		case 3: // #rgb → #rrggbb
			hex = string([]byte{
				hex[0], hex[0],
				hex[1], hex[1],
				hex[2], hex[2],
			})
		case 4: // #rgba → #rrggbb (drop alpha)
			hex = string([]byte{
				hex[0], hex[0],
				hex[1], hex[1],
				hex[2], hex[2],
			})
		case 6: // already 6 digits
		case 8: // #rrggbbaa → drop alpha
			hex = hex[:6]
		default:
			return "[white]"
		}
		return "[#" + hex + "]"
	}
	// Named color — wrap in brackets
	return "[" + strings.ToLower(s) + "]"
}

// ValidNamedColors is the list of named colors users can choose via /user_color.
var ValidNamedColors = []string{
	"red", "green", "blue", "cyan", "magenta", "yellow",
	"white", "orange", "purple", "teal", "lime", "pink",
}

// IsValidNamedColor returns true if s is a supported named color.
func IsValidNamedColor(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	for _, c := range ValidNamedColors {
		if c == s {
			return true
		}
	}
	return false
}
