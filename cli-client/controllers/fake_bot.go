package controllers

import (
	"log"
	"sync/atomic"
	"time"

	"cli-client/views"

	"github.com/rivo/tview"
)

type FakeBot struct {
	stop    chan struct{}
	app     *tview.Application
	stopped int32 // atomic flag — 1 means stopped
}

func NewFakeBot(app *tview.Application) *FakeBot {
	return &FakeBot{
		stop: make(chan struct{}),
		app:  app,
	}
}

func (b *FakeBot) Start(chat *views.ChatView) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("PANIC in FakeBot goroutine: %v", r)
			}
		}()

		// Each fake message mirrors the JSON wire format:
		//   { "user": "...", "message": "...", "color": "..." }
		// Color is a tview tag or hex like "#rrggbb" — ParseColorToTag handles both.
		messages := []struct {
			user  string
			text  string
			color string
		}{
			{"cyber_punk", "Hey! Welcome to the global chat!", "[green]"},
			{"gopher_dev", "Nice TUI. Very clean layout.", "[magenta]"},
			{"script_kiddie", "Anyone using Go 1.22 yet?", "[yellow]"},
			{"script_kiddie", "Anyone using Go 1.22 yet?", "[yellow]"},
			{"script_kiddie", "Anyone using Go 1.22 yet?", "[yellow]"},
			{"script_kiddie", "Anyone using Go 1.22 yet?", "[yellow]"},
			{"script_kiddie", "Anyone using Go 1.22 yet?", "[yellow]"},
			{"script_kiddie", "Anyone using Go 1.22 yet?", "[yellow]"},
			{"script_kiddie", "Anyone using Go 1.22 yet?", "[yellow]"},
			{"script_kiddie", "Anyone using Go 1.22 yet?", "[yellow]"},
			{"anon_x", "Latency is pretty good for a global node.", "[green]"},
			{"gopher_dev", "Optimizing for iOS Termux users. Keep it minimal.", "[magenta]"},
		}

		for _, msg := range messages {
			select {
			case <-b.stop:
				return
			case <-time.After(2 * time.Second):
				if atomic.LoadInt32(&b.stopped) == 1 {
					return
				}
				// AddIncomingMessage already calls QueueUpdateDraw internally —
				// do NOT wrap in an outer QueueUpdateDraw (that would nest them).
				chat.AddIncomingMessage(msg.user, msg.text, msg.color)
			}
		}
	}()
}

func (b *FakeBot) Stop() {
	// Mark stopped BEFORE closing channel so goroutines see the flag immediately.
	atomic.StoreInt32(&b.stopped, 1)
	if b.stop != nil {
		close(b.stop)
	}
}
