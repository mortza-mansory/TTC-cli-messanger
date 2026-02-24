package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"secure-chat-backend/internal/controllers"
	"secure-chat-backend/internal/middleware"
	"secure-chat-backend/internal/models"
	"secure-chat-backend/internal/services"
)

type Server struct {
	chatController  *controllers.SendController
	pollController  *controllers.PollController
	statsController *controllers.StatsController

	loggingMiddleware  *middleware.LoggingMiddleware
	recoveryMiddleware *middleware.RecoveryMiddleware
	corsMiddleware     *middleware.CORSMiddleware

	chatService *services.ChatService
	authService *services.AuthService

	httpServer *http.Server
	config     *Config
}

type Config struct {
	Port            string
	AccessKey       string
	MaxMessages     int
	MessageTTL      time.Duration
	CleanupInterval time.Duration
}

func NewServer(config *Config) *Server {
	buffer := models.NewMessageBuffer(config.MaxMessages, config.MessageTTL)

	chatService := services.NewChatService(buffer)
	authService := services.NewAuthService(config.AccessKey)

	authService.CleanupOldClients(24 * time.Hour)

	chatController := controllers.NewSendController(chatService, authService)
	pollController := controllers.NewPollController(chatService, authService)
	statsController := controllers.NewStatsController(chatService, authService)

	loggingMiddleware := middleware.NewLoggingMiddleware()
	recoveryMiddleware := middleware.NewRecoveryMiddleware()
	corsMiddleware := middleware.NewCORSMiddleware()

	return &Server{
		chatController:     chatController,
		pollController:     pollController,
		statsController:    statsController,
		loggingMiddleware:  loggingMiddleware,
		recoveryMiddleware: recoveryMiddleware,
		corsMiddleware:     corsMiddleware,
		chatService:        chatService,
		authService:        authService,
		config:             config,
	}
}

func (s *Server) registerRoutes() {
	wrap := func(handler http.HandlerFunc) http.HandlerFunc {
		return s.recoveryMiddleware.Wrap(
			s.loggingMiddleware.Wrap(
				s.corsMiddleware.Wrap(handler),
			),
		)
	}

	http.HandleFunc("/api/send", wrap(s.chatController.Handle))
	http.HandleFunc("/api/poll", wrap(s.pollController.Handle))
	http.HandleFunc("/api/stats", wrap(s.statsController.Handle))

	http.HandleFunc("/health", wrap(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
}

func (s *Server) Start() error {
	s.registerRoutes()

	s.httpServer = &http.Server{
		Addr:         ":" + s.config.Port,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	log.Printf("Server started on port %s", s.config.Port)
	log.Printf("Access Key: %s", s.config.AccessKey)
	log.Printf("Max Messages: %d, Message TTL: %v", s.config.MaxMessages, s.config.MessageTTL)

	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown() error {
	log.Println("Initializing server shutdown...")
	if s.httpServer != nil {
		return s.httpServer.Close()
	}
	return nil
}

func main() {
	port := flag.String("port", "8034", "Port to run the server on")
	accessKey := flag.String("key", "secure_chat_key_2024", "Access key for clients")
	maxMessages := flag.Int("max-msgs", 1000, "Maximum number of messages to store")
	msgTTL := flag.Duration("ttl", 1*time.Minute, "Time to live for messages")
	flag.Parse()

	config := &Config{
		Port:            *port,
		AccessKey:       *accessKey,
		MaxMessages:     *maxMessages,
		MessageTTL:      *msgTTL,
		CleanupInterval: 10 * time.Second,
	}

	server := NewServer(config)

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		fmt.Println()
		log.Println("Received shutdown signal, exiting...")

		if err := server.Shutdown(); err != nil {
			log.Printf("Error shutting down server: %v", err)
		}

		os.Exit(0)
	}()

	if err := server.Start(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Error starting server: %v", err)
	}
}
