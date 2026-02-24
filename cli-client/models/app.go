package models

// AppState represents the overall application state
type AppState struct {
	CurrentUser *User
	Messages    []*Message
	Users       map[string]*User
	UserColors  map[string]string // username → tview color tag override e.g. "[#ff00ff]"
	Latency     int
	IsConnected bool
}

// NewAppState creates a new application state
func NewAppState() *AppState {
	return &AppState{
		CurrentUser: nil,
		Messages:    make([]*Message, 0),
		Users:       make(map[string]*User),
		UserColors:  make(map[string]string),
		Latency:     18,
		IsConnected: true,
	}
}

// AddMessage adds a message to the chat
func (a *AppState) AddMessage(msg *Message) {
	a.Messages = append(a.Messages, msg)
}

// GetMessages returns all messages
func (a *AppState) GetMessages() []*Message {
	return a.Messages
}

// SetCurrentUser sets the current user
func (a *AppState) SetCurrentUser(username string) {
	a.CurrentUser = NewUser(username)
	a.Users[username] = a.CurrentUser
}

// GetUserColorTag returns the tview color tag for a user.
// Checks the manual override map first; falls back to the hash-based default.
func (a *AppState) GetUserColorTag(username string) string {
	if tag, ok := a.UserColors[username]; ok {
		return tag
	}
	return GetUsernameColor(username)
}

// SetUserColor stores a manual color override for a user.
// colorTag must be a valid tview tag e.g. "[green]" or "[#ff00ff]".
func (a *AppState) SetUserColor(username, colorTag string) {
	a.UserColors[username] = colorTag
	// Keep the User struct in sync if it exists
	if u, ok := a.Users[username]; ok {
		u.Color = colorTag
	}
	if a.CurrentUser != nil && a.CurrentUser.Username == username {
		a.CurrentUser.Color = colorTag
	}
}

// GetOnlineUsersCount returns the count of online users
func (a *AppState) GetOnlineUsersCount() int {
	count := 0
	for _, u := range a.Users {
		if u.IsOnline {
			count++
		}
	}
	if a.CurrentUser != nil {
		count++
	}
	if count == 0 {
		count = 1
	}
	return count
}
