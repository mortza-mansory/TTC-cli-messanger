// internal/controllers/send_controller.go
package controllers

import (
	"encoding/json"
	"net/http"
	"time"

	"secure-chat-backend/internal/services"
)

// SendController کنترلر ارسال پیام
type SendController struct {
	chatService *services.ChatService
	authService *services.AuthService
}

// SendRequest ساختار درخواست با فرمت جدید
type SendRequest struct {
	AccessKey string `json:"access_key"`
	ClientID  string `json:"client_id"`
	Username  string `json:"username"` // مثلا "script_kiddie"
	Content   string `json:"content"`  // متن پیام
	Color     string `json:"color"`    // مثل "[yellow]"
}

// SendResponse ساختار پاسخ
type SendResponse struct {
	Status string `json:"status"`
	ID     string `json:"id"`
	Time   string `json:"time"`
}

// NewSendController سازنده
func NewSendController(chatService *services.ChatService, authService *services.AuthService) *SendController {
	return &SendController{
		chatService: chatService,
		authService: authService,
	}
}

// Handle پردازش درخواست ارسال
func (c *SendController) Handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// اعتبارسنجی
	if !c.authService.ValidateAccess(req.AccessKey, req.ClientID) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if !c.authService.CheckRateLimit(req.ClientID) {
		http.Error(w, "Too many requests", http.StatusTooManyRequests)
		return
	}

	// تنظیم رنگ پیش‌فرض اگر خالی بود
	if req.Color == "" {
		req.Color = "[white]"
	}

	// ارسال پیام
	msg, err := c.chatService.SendMessage(req.Username, req.Content, req.Color, req.ClientID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(SendResponse{
		Status: "sent",
		ID:     msg.ID,
		Time:   time.Now().Format(time.RFC3339),
	})
}
