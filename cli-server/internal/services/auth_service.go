package services

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type AuthService struct {
	accessKey    string
	mu           sync.RWMutex
	clients      map[string]*ClientInfo
	rateLimiters map[string]*rate.Limiter
	rateLimit    rate.Limit
	rateBurst    int
}

type ClientInfo struct {
	ID           string
	FirstSeen    time.Time
	LastSeen     time.Time
	MessageCount int64
}

func NewAuthService(accessKey string) *AuthService {
	return &AuthService{
		accessKey:    accessKey,
		clients:      make(map[string]*ClientInfo),
		rateLimiters: make(map[string]*rate.Limiter),
		rateLimit:    10,
		rateBurst:    20,
	}
}

func (s *AuthService) ValidateAccess(key, clientID string) bool {
	if key != s.accessKey {
		return false
	}

	if clientID == "" {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	if client, exists := s.clients[clientID]; exists {
		client.LastSeen = now
		client.MessageCount++
	} else {
		s.clients[clientID] = &ClientInfo{
			ID:           clientID,
			FirstSeen:    now,
			LastSeen:     now,
			MessageCount: 1,
		}
		s.rateLimiters[clientID] = rate.NewLimiter(s.rateLimit, s.rateBurst)
	}

	return true
}

func (s *AuthService) CheckRateLimit(clientID string) bool {
	s.mu.RLock()
	limiter, exists := s.rateLimiters[clientID]
	s.mu.RUnlock()

	if !exists {
		return true
	}

	return limiter.Allow()
}

func (s *AuthService) CleanupOldClients(maxAge time.Duration) {
	ticker := time.NewTicker(5 * time.Minute)
	go func() {
		for range ticker.C {
			s.mu.Lock()
			now := time.Now()
			for id, client := range s.clients {
				if now.Sub(client.LastSeen) > maxAge {
					delete(s.clients, id)
					delete(s.rateLimiters, id)
				}
			}
			s.mu.Unlock()
		}
	}()
}

func (s *AuthService) GetClientCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.clients)
}
