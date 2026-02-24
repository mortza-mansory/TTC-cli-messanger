package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"cli-client/controllers"
	"cli-client/models"
	"cli-client/views"

	"github.com/rivo/tview"
)

var logFile *os.File

func init() {
	var err error
	logFile, err = os.Create("error.txt")
	if err != nil {
		fmt.Println("Failed to create error log file:", err)
	}
}

func logError(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logLine := fmt.Sprintf("[%s] ERROR: %s\n", timestamp, msg)
	fmt.Print(logLine)
	if logFile != nil {
		logFile.WriteString(logLine)
		logFile.Sync()
	}
}

func recoverFromPanic() {
	if r := recover(); r != nil {
		logError("PANIC RECOVERED: %v", r)
	}
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			logError("FATAL PANIC in main: %v", r)
			if logFile != nil {
				logFile.Close()
			}
			os.Exit(1)
		}
	}()

	app := tview.NewApplication()
	pages := tview.NewPages()

	ctrl := controllers.NewAppController(app)

	loadingView := views.NewLoadingView(app)
	loginView := views.NewLoginView(app, ctrl.OnLoginSubmit)
	chatView := views.NewChatView(
		app,
		ctrl.OnSendMessage,
		ctrl.OnCommand,
	)

	ctrl.RegisterView(models.ScreenLoading, loadingView)
	ctrl.RegisterView(models.ScreenLogin, loginView)
	ctrl.RegisterView(models.ScreenChat, chatView)

	pages.AddPage("loading", loadingView.GetPrimitive(), true, true)
	pages.AddPage("login", loginView.Primitive(), true, false)
	pages.AddPage("chat", chatView.Primitive(), true, false)

	// ── LOADING ───────────────────────────────────────────────────────────────
	ctrl.SM.OnEnter(models.ScreenLoading, func() {
		defer recoverFromPanic()
		pages.SwitchToPage("loading")

		go func() {
			defer recoverFromPanic()

			// ── Phase 1: animate the progress bar ─────────────────────────────
			steps := []struct {
				progress int
				label    string
			}{
				{10, "Initializing…"},
				{20, "Loading modules…"},
				{40, "Preparing encryption…"},
				{60, "Checking configuration…"},
				{80, "Contacting relay server…"},
				{90, "Verifying connection…"},
				{100, ""},
			}
			for _, s := range steps {
				time.Sleep(140 * time.Millisecond)
				loadingView.UpdateProgress(s.progress)
				if s.label != "" {
					loadingView.SetStatus(s.label)
				}
			}

			// ── Phase 2: real connectivity check ──────────────────────────────
			loadingView.SetStatus("Contacting relay server…")
			connErr := controllers.CheckServerConnectivity(controllers.DefaultServerURL)

			if connErr != nil {
				// ── FAILURE PATH ──────────────────────────────────────────────
				// Paint the fatal error banner immediately.
				app.QueueUpdateDraw(func() {
					defer recoverFromPanic()
					loadingView.ShowFatalError(
						fmt.Sprintf("Server not reachable — %s", controllers.DefaultServerURL),
					)
					loadingView.SetCountdown(4)
				})

				// Tick the countdown 4 → 3 → 2 → 1 then exit.
				for i := 3; i >= 0; i-- {
					time.Sleep(1 * time.Second)
					remaining := i
					app.QueueUpdateDraw(func() {
						defer recoverFromPanic()
						if remaining == 0 {
							loadingView.SetCountdown(0)
						} else {
							loadingView.SetCountdown(remaining)
						}
					})
				}

				// Give the last frame one render cycle then stop the app.
				time.Sleep(200 * time.Millisecond)
				app.Stop()
				return
			}

			// ── SUCCESS PATH ──────────────────────────────────────────────────
			loadingView.SetStatus("Connected  ✓")
			time.Sleep(300 * time.Millisecond)

			app.QueueUpdateDraw(func() {
				defer recoverFromPanic()
				ctrl.SM.Transition(models.ScreenLogin)
			})
		}()
	})

	// ── LOGIN ─────────────────────────────────────────────────────────────────
	ctrl.SM.OnEnter(models.ScreenLogin, func() {
		defer recoverFromPanic()
		pages.SwitchToPage("login")
		loginView.StartUsernamePrompt()
		app.SetFocus(loginView.Primitive())
	})

	// ── CHAT ──────────────────────────────────────────────────────────────────
	ctrl.SM.OnEnter(models.ScreenChat, func() {
		defer recoverFromPanic()
		pages.SwitchToPage("chat")
		app.SetFocus(chatView.InputPrimitive())
	})

	// ── CHAT EXIT ─────────────────────────────────────────────────────────────
	ctrl.SM.OnExit(models.ScreenChat, func() {
		defer recoverFromPanic()
		ctrl.StopBot()
		if chat, ok := ctrl.Views[models.ScreenChat].(*views.ChatView); ok {
			chat.Stop()
		}
	})

	// Kick off the state machine from a goroutine so the tview event loop is
	// already running by the time the first Transition fires.
	go func() {
		defer recoverFromPanic()
		time.Sleep(100 * time.Millisecond)
		app.QueueUpdateDraw(func() {
			defer recoverFromPanic()
			ctrl.SM.Transition(models.ScreenLoading)
		})
	}()

	if err := app.SetRoot(pages, true).Run(); err != nil {
		logError("Application error: %v", err)
		log.Printf("Application error: %v", err)
	}

	if logFile != nil {
		logFile.Close()
	}
}
