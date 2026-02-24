// internal/controllers/poll_controller.go
package controllers

import (
	"encoding/json"
	"net/http"
	"time"

	"secure-chat-backend/internal/services"
)

// PollController کنترلر long polling
type PollController struct {
	chatService *services.ChatService
	authService *services.AuthService
	pollTimeout time.Duration
}

// NewPollController سازنده
func NewPollController(chatService *services.ChatService, authService *services.AuthService) *PollController {
	return &PollController{
		chatService: chatService,
		authService: authService,
		pollTimeout: 30 * time.Second,
	}
}

// Handle پردازش درخواست long polling
func (c *PollController) Handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	accessKey := r.URL.Query().Get("access_key")
	clientID := r.URL.Query().Get("client_id")
	lastID := r.URL.Query().Get("last_id")

	if !c.authService.ValidateAccess(accessKey, clientID) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	messages, err := c.chatService.WaitForMessages(clientID, lastID, c.pollTimeout)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if len(messages) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// تبدیل پیام‌ها به فرمت مورد نظر کلاینت
	response := make([]map[string]interface{}, len(messages))
	for i, msg := range messages {
		response[i] = msg.ToClientFormat()
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
