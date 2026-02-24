package views

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type LoadingView struct {
	app          *tview.Application
	container    *tview.Flex
	progressText *tview.TextView
	statusText   *tview.TextView
	errorText    *tview.TextView // shown only on fatal error
	animFrame    int
}

func NewLoadingView(app *tview.Application) *LoadingView {
	l := &LoadingView{app: app}
	l.buildUI()
	return l
}

func (l *LoadingView) buildUI() {
	logoText := tview.NewTextView()
	logoText.SetDynamicColors(true)
	logoText.SetTextAlign(tview.AlignCenter)
	logoText.SetText(
		"[cyan]╔═══════════════════════════════════════╗\n" +
			"║        SecTherminal  v1.0.0           ║\n" +
			"║     Secure  ·  Fast  ·  Open          ║\n" +
			"╚═══════════════════════════════════════╝[-]",
	)

	l.progressText = tview.NewTextView()
	l.progressText.SetDynamicColors(true)
	l.progressText.SetTextAlign(tview.AlignCenter)
	l.progressText.SetText("[green]░░░░░░░░░░░░░░░░░░░░[-]  0%")

	l.statusText = tview.NewTextView()
	l.statusText.SetDynamicColors(true)
	l.statusText.SetTextAlign(tview.AlignCenter)
	l.statusText.SetText("[dim]Initializing…[-]")

	// errorText is invisible until ShowFatalError is called.
	l.errorText = tview.NewTextView()
	l.errorText.SetDynamicColors(true)
	l.errorText.SetTextAlign(tview.AlignCenter)
	l.errorText.SetText("")
	l.errorText.SetBackgroundColor(tcell.ColorBlack)

	l.container = tview.NewFlex()
	l.container.SetDirection(tview.FlexRow)
	l.container.SetBackgroundColor(tcell.ColorBlack)
	l.container.AddItem(logoText, 0, 1, false)
	l.container.AddItem(tview.NewBox().SetBackgroundColor(tcell.ColorBlack), 1, 0, false)
	l.container.AddItem(l.progressText, 1, 0, false)
	l.container.AddItem(l.statusText, 1, 0, false)
	l.container.AddItem(tview.NewBox().SetBackgroundColor(tcell.ColorBlack), 1, 0, false)
	l.container.AddItem(l.errorText, 3, 0, false) // 3 lines: gap + error + countdown
}

func (l *LoadingView) GetPrimitive() tview.Primitive {
	return l.container
}

// UpdateProgress redraws the progress bar. Safe to call from any goroutine.
func (l *LoadingView) UpdateProgress(progress int) {
	l.app.QueueUpdateDraw(func() {
		filled := progress / 5
		empty := 20 - filled
		bar := ""
		for i := 0; i < filled; i++ {
			bar += "█"
		}
		for i := 0; i < empty; i++ {
			bar += "░"
		}
		l.progressText.SetText(fmt.Sprintf("[green]%s[-]  %d%%", bar, progress))
	})
}

// SetStatus updates the small status line under the progress bar.
// Safe to call from any goroutine.
func (l *LoadingView) SetStatus(text string) {
	l.app.QueueUpdateDraw(func() {
		l.statusText.SetText(fmt.Sprintf("[dim]%s[-]", text))
	})
}

// ShowFatalError replaces the status line with a red error banner.
// Call SetCountdown immediately after to start the countdown ticker.
// Must be called via QueueUpdateDraw (or from within the event loop).
func (l *LoadingView) ShowFatalError(message string) {
	// Freeze the progress bar in red to signal failure.
	l.progressText.SetText("[red]████████████████████[-]  ERROR")
	l.statusText.SetText("")
	l.errorText.SetText(fmt.Sprintf(
		"[red]✗  %s[-]",
		message,
	))
}

// SetCountdown updates the countdown line inside the error area.
// Must be called via QueueUpdateDraw (or from within the event loop).
func (l *LoadingView) SetCountdown(seconds int) {
	current := l.errorText.GetText(false)
	// Keep only the first line (the error message itself) and replace line 2.
	lines := splitFirstLine(current)
	dots := ""
	for i := 0; i < seconds; i++ {
		dots += "●"
	}
	for i := seconds; i < 4; i++ {
		dots += "○"
	}
	l.errorText.SetText(fmt.Sprintf(
		"%s\n[dim]Exiting in %d second%s…  %s[-]",
		lines, seconds, pluralS(seconds), dots,
	))
}

// ── helpers ───────────────────────────────────────────────────────────────────

func splitFirstLine(s string) string {
	for i, ch := range s {
		if ch == '\n' {
			return s[:i]
		}
	}
	return s
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
