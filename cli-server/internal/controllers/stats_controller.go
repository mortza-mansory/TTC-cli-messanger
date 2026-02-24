package controllers

import (
	"encoding/json"
	"net/http"

	"secure-chat-backend/internal/services"
)

type StatsController struct {
	chatService *services.ChatService
	authService *services.AuthService
}

func NewStatsController(chatService *services.ChatService, authService *services.AuthService) *StatsController {
	return &StatsController{
		chatService: chatService,
		authService: authService,
	}
}

func (c *StatsController) Handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats := map[string]interface{}{
		"chat_stats":     c.chatService.GetStats(),
		"active_clients": c.authService.GetClientCount(),
		"status":         "running",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}
