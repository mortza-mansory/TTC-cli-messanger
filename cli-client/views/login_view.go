package views

import (
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type LoginView struct {
	app         *tview.Application
	container   *tview.Flex
	headerBox   *tview.Box
	textView    *tview.TextView
	inputField  *tview.InputField
	onSubmit    func(username, password string)
	currentStep int
	username    string
}

func NewLoginView(
	app *tview.Application,
	onSubmit func(string, string),
) *LoginView {
	l := &LoginView{
		app:         app,
		onSubmit:    onSubmit,
		currentStep: 0,
	}
	l.buildUI()
	return l
}
func (l *LoginView) Primitive() tview.Primitive {
	return l.container
}
func (l *LoginView) buildUI() {
	l.headerBox = tview.NewBox()
	l.headerBox.SetBorder(true)
	l.headerBox.SetTitle(" TERMINAL MESSENGER v1.0.0 ")
	l.headerBox.SetBackgroundColor(tcell.ColorBlack)

	l.textView = tview.NewTextView()
	l.textView.SetDynamicColors(true)
	l.textView.SetTextAlign(tview.AlignLeft)
	l.textView.SetBackgroundColor(tcell.ColorBlack)

	l.inputField = tview.NewInputField()
	l.inputField.SetLabel("> ")
	l.inputField.SetPlaceholder("Type here...")
	l.inputField.SetFieldBackgroundColor(tcell.ColorBlack)
	l.inputField.SetFieldTextColor(tcell.ColorWhite)
	l.inputField.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			l.handleEnter()
		}
	})

	l.container = tview.NewFlex()
	l.container.SetDirection(tview.FlexRow)
	l.container.SetBackgroundColor(tcell.ColorBlack)
	l.container.AddItem(l.headerBox, 3, 0, false)
	l.container.AddItem(l.textView, 0, 1, false)
	l.container.AddItem(l.inputField, 1, 0, true)
}

func (l *LoginView) handleEnter() {
	text := l.inputField.GetText()
	if text == "" {
		return
	}
	l.inputField.SetText("")

	if l.currentStep == 0 {
		l.username = text
		l.currentStep = 1
		l.typewriterText("\n[cyan]Password: [white]")
	} else {
		l.onSubmit(l.username, text)
	}
}

// typewriterText displays text with fast character-by-character animation
// preserving color codes and newlines
func (l *LoginView) typewriterText(text string) {
	go func() {
		for _, char := range text {
			l.app.QueueUpdateDraw(func() {
				current := l.textView.GetText(false)
				l.textView.SetText(current + string(char))
			})
			time.Sleep(10 * time.Millisecond) // Fast for high-tech feel
		}
	}()
}

func (l *LoginView) StartUsernamePrompt() {
	l.currentStep = 0
	l.typewriterText(`[yellow]! Establishing secure connection...[white]
[green]✓ Connection established.[white]

[cyan]Tell us your username:[white] `)
}

func (l *LoginView) GetPrimitive() tview.Primitive {
	return l.container
}
