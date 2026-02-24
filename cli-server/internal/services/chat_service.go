package services

import (
	"errors"
	"sync"
	"time"

	"secure-chat-backend/internal/models"
	"secure-chat-backend/internal/utils"
)

type ChatService struct {
	buffer     *models.MessageBuffer
	mu         sync.RWMutex
	waiters    map[string]chan struct{}
	maxWaiters int
	msgCounter int64
}

func NewChatService(buffer *models.MessageBuffer) *ChatService {
	return &ChatService{
		buffer:     buffer,
		waiters:    make(map[string]chan struct{}),
		maxWaiters: 1000,
		msgCounter: 0,
	}
}

func (s *ChatService) SendMessage(username, content, color, clientID string) (*models.Message, error) {
	if username == "" || content == "" {
		return nil, errors.New("username and content cannot be empty")
	}

	if color != "" && !utils.IsValidColor(color) {
		color = "[white]"
	}

	s.msgCounter++
	msgID := utils.GenerateID()

	msg := &models.Message{
		ID:        msgID,
		Username:  username,
		Content:   content,
		Color:     color,
		Timestamp: time.Now(),
	}

	s.buffer.Add(msg)

	s.notifyWaiters()

	return msg, nil
}

func (s *ChatService) GetMessages(afterID string) ([]*models.Message, error) {
	return s.buffer.GetAfter(afterID, 50), nil
}

func (s *ChatService) WaitForMessages(clientID, afterID string, timeout time.Duration) ([]*models.Message, error) {
	if messages := s.buffer.GetAfter(afterID, 50); len(messages) > 0 {
		return messages, nil
	}

	waiter := make(chan struct{}, 1)

	s.mu.Lock()
	if len(s.waiters) >= s.maxWaiters {
		s.mu.Unlock()
		return nil, errors.New("server is busy")
	}
	s.waiters[clientID] = waiter
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.waiters, clientID)
		s.mu.Unlock()
		close(waiter)
	}()

	select {
	case <-waiter:
		return s.buffer.GetAfter(afterID, 50), nil
	case <-time.After(timeout):
		return []*models.Message{}, nil
	}
}

func (s *ChatService) notifyWaiters() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, waiter := range s.waiters {
		select {
		case waiter <- struct{}{}:
		default:
		}
	}
}

func (s *ChatService) GetStats() map[string]interface{} {
	s.mu.RLock()
	waiterCount := len(s.waiters)
	s.mu.RUnlock()

	return map[string]interface{}{
		"total_messages":  s.buffer.Len(),
		"waiting_clients": waiterCount,
		"max_waiters":     s.maxWaiters,
	}
}
