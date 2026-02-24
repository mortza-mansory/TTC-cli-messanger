package models

import (
	"encoding/json"
	"sync"
	"time"
)

type Message struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Content   string    `json:"content"`
	Color     string    `json:"color"`
	Timestamp time.Time `json:"timestamp"`
	ExpireAt  time.Time `json:"-"`
}

func (m *Message) MarshalJSON() ([]byte, error) {
	msgMap := map[string]interface{}{
		m.Username:  m.Content,
		"color":     m.Color,
		"id":        m.ID,
		"timestamp": m.Timestamp.Format(time.RFC3339),
	}
	return json.Marshal(msgMap)
}

func (m *Message) ToClientFormat() map[string]interface{} {
	return map[string]interface{}{
		m.Username: m.Content,
		"color":    m.Color,
		"id":       m.ID,
	}
}

type MessageBuffer struct {
	mu       sync.RWMutex
	messages []*Message
	maxSize  int
	ttl      time.Duration
}

func NewMessageBuffer(maxSize int, ttl time.Duration) *MessageBuffer {
	mb := &MessageBuffer{
		messages: make([]*Message, 0, maxSize),
		maxSize:  maxSize,
		ttl:      ttl,
	}

	go mb.cleanupLoop()

	return mb
}

func (mb *MessageBuffer) Add(msg *Message) {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	msg.ExpireAt = time.Now().Add(mb.ttl)
	mb.messages = append(mb.messages, msg)

	if len(mb.messages) > mb.maxSize {
		mb.messages = mb.messages[1:]
	}
}

func (mb *MessageBuffer) GetAfter(afterID string, limit int) []*Message {
	mb.mu.RLock()
	defer mb.mu.RUnlock()

	if afterID == "" {
		return mb.getLastMessages(limit)
	}

	startIdx := -1
	for i, msg := range mb.messages {
		if msg.ID == afterID {
			startIdx = i + 1
			break
		}
	}

	if startIdx < 0 || startIdx >= len(mb.messages) {
		return []*Message{}
	}

	result := make([]*Message, len(mb.messages)-startIdx)
	copy(result, mb.messages[startIdx:])
	return result
}

func (mb *MessageBuffer) getLastMessages(limit int) []*Message {
	if len(mb.messages) == 0 {
		return []*Message{}
	}

	if len(mb.messages) <= limit {
		result := make([]*Message, len(mb.messages))
		copy(result, mb.messages)
		return result
	}

	result := make([]*Message, limit)
	copy(result, mb.messages[len(mb.messages)-limit:])
	return result
}

func (mb *MessageBuffer) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Second)
	for range ticker.C {
		mb.mu.Lock()
		now := time.Now()
		newMessages := make([]*Message, 0, len(mb.messages))
		for _, msg := range mb.messages {
			if msg.ExpireAt.After(now) {
				newMessages = append(newMessages, msg)
			}
		}
		mb.messages = newMessages
		mb.mu.Unlock()
	}
}

func (mb *MessageBuffer) Len() int {
	mb.mu.RLock()
	defer mb.mu.RUnlock()
	return len(mb.messages)
}
