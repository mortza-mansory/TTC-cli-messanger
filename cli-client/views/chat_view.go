package views

import (
	"fmt"
	"log"
	"strings"
	"sync/atomic"
	"time"

	"cli-client/models"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type ChatView struct {
	app           *tview.Application
	container     *tview.Flex
	header        *tview.TextView
	messageView   *tview.TextView
	inputField    *tview.InputField
	footer        *tview.TextView
	commandBar    *tview.TextView
	onSendMessage func(string)
	onCommand     func(string)

	stopped  int32 // atomic: 1 = stopped
	animMode int32 // atomic: 1 = word-by-word, 0 = static

	// Header state — only touched inside tview event loop
	headerUsername string
	headerLatency  int
	headerOnline   bool

	// Nick mode / message history — only touched inside tview event loop
	nickActive  bool
	sentHistory []string
	historyIdx  int // -1 = not browsing

	// ── Message render model ──────────────────────────────────────────────
	// All fields below are ONLY ever read/written from inside QueueUpdateDraw
	// (i.e. the tview event loop), so no mutex is needed.
	//
	// Design: the visible text is always:
	//   committedText  +  inFlight[0] + inFlight[1] + ...   (by insertion order)
	//
	// AddMessage      → appends a fully-formatted line to committedText, re-renders.
	// Animation start → allocates an inFlight slot (animID), re-renders.
	// Animation tick  → updates the slot text, re-renders.
	// Animation end   → moves final line from slot into committedText, re-renders.
	//
	// Because AddMessage only touches committedText (never overwrites inFlight),
	// and animations only touch their own slot, messages never clobber each other.
	committedText string
	inFlight      map[int]string // animID → current partial line (with trailing cursor)
	nextAnimID    int            // monotonically increasing; never resets
	inFlightGen   int            // incremented by ClearMessages; stale callbacks bail out
}

func NewChatView(
	app *tview.Application,
	onSendMessage func(string),
	onCommand func(string),
) *ChatView {
	c := &ChatView{
		app:           app,
		onSendMessage: onSendMessage,
		onCommand:     onCommand,
		historyIdx:    -1,
		headerLatency: 18,
		headerOnline:  true,
		inFlight:      make(map[int]string),
	}
	atomic.StoreInt32(&c.animMode, 1)
	c.buildUI()
	c.startClockTicker()
	return c
}

func (c *ChatView) Primitive() tview.Primitive      { return c.container }
func (c *ChatView) InputPrimitive() tview.Primitive { return c.inputField }
func (c *ChatView) GetPrimitive() tview.Primitive   { return c.container }

// ── UI construction ────────────────────────────────────────────────────────

func (c *ChatView) buildUI() {
	// Header — bordered box, cyan border to match the project theme.
	// Height 3 in the flex (1 top border + 1 content line + 1 bottom border).
	c.header = tview.NewTextView()
	c.header.SetDynamicColors(true)
	c.header.SetTextAlign(tview.AlignLeft)
	c.header.SetBackgroundColor(tcell.ColorBlack)
	c.header.SetBorder(true)
	c.header.SetBorderColor(tcell.ColorDarkCyan)
	c.header.SetBorderPadding(0, 0, 1, 1)

	c.messageView = tview.NewTextView()
	c.messageView.SetDynamicColors(true)
	c.messageView.SetScrollable(true)
	c.messageView.SetWordWrap(true)
	c.messageView.SetText("")
	c.messageView.SetBackgroundColor(tcell.ColorBlack)

	c.commandBar = tview.NewTextView()
	c.commandBar.SetDynamicColors(true)
	c.commandBar.SetTextAlign(tview.AlignLeft)
	c.commandBar.SetBackgroundColor(tcell.ColorBlack)
	c.redrawCommandBar()

	c.inputField = tview.NewInputField()
	c.inputField.SetLabel("  > ")
	c.inputField.SetPlaceholder("Type a message or /command...")
	c.inputField.SetFieldBackgroundColor(tcell.ColorBlack)
	c.inputField.SetFieldTextColor(tcell.ColorWhite)
	c.inputField.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			text := c.inputField.GetText()
			if text != "" {
				if strings.HasPrefix(text, "/") {
					c.onCommand(text)
				} else {
					c.onSendMessage(text)
				}
				c.inputField.SetText("")
				c.historyIdx = -1
			}
		}
	})

	// ── Arrow-key capture for nick-mode history navigation ─────────────────
	// When nick mode is OFF  → keys behave normally.
	// When nick mode is ON:
	//   ← (Left)  → go to previous (older) sent message.
	//               Only activates when the field is empty OR already in history,
	//               so normal left-cursor movement still works while typing fresh text.
	//   → (Right) → go to next (newer) sent message / clears at the newest end.
	c.inputField.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if !c.nickActive {
			return event
		}
		fieldEmpty := c.inputField.GetText() == ""
		inHistory := c.historyIdx >= 0

		switch event.Key() {
		case tcell.KeyLeft:
			if !fieldEmpty && !inHistory {
				return event // editing a fresh message — let cursor move
			}
			if len(c.sentHistory) == 0 {
				return nil
			}
			if c.historyIdx < 0 {
				c.historyIdx = len(c.sentHistory) - 1
			} else if c.historyIdx > 0 {
				c.historyIdx--
			}
			c.inputField.SetText(c.sentHistory[c.historyIdx])
			return nil // consumed

		case tcell.KeyRight:
			if !fieldEmpty && !inHistory {
				return event // editing a fresh message — let cursor move
			}
			if c.historyIdx < 0 {
				return nil
			}
			c.historyIdx++
			if c.historyIdx >= len(c.sentHistory) {
				c.historyIdx = -1
				c.inputField.SetText("")
			} else {
				c.inputField.SetText(c.sentHistory[c.historyIdx])
			}
			return nil // consumed
		}
		return event
	})

	c.footer = tview.NewTextView()
	c.footer.SetDynamicColors(true)
	c.footer.SetTextAlign(tview.AlignLeft)
	c.footer.SetText("[magenta]NORMAL[white]    SecTherminal              UTF-8    L:1, C:1")
	c.footer.SetBackgroundColor(tcell.ColorBlack)

	c.container = tview.NewFlex()
	c.container.SetDirection(tview.FlexRow)
	c.container.SetBackgroundColor(tcell.ColorBlack)
	c.container.AddItem(c.header, 3, 0, false) // 3 = border top + 1 line + border bottom
	c.container.AddItem(c.messageView, 0, 1, false)
	c.container.AddItem(c.commandBar, 1, 0, false)
	c.container.AddItem(c.inputField, 3, 0, true)
	c.container.AddItem(c.footer, 1, 0, false)

	c.redrawHeader()
}

// ── Message render engine ──────────────────────────────────────────────────

// sanitizeContent escapes raw user-supplied text for safe rendering inside
// a tview TextView with SetDynamicColors(true).
//
// tview treats anything matching `[word]` as a color/style tag. User messages
// can contain arbitrary `[` characters (URLs, code snippets, IRC nicks like
// `[nick]`). An unmatched or unrecognised `[` sequence causes tview to panic
// with an index-out-of-bounds — a fatal error that recover() cannot catch.
//
// The fix: replace every `[` in user content with `[[]` (tview's own escape
// for a literal `[`). We do NOT escape color tags we intentionally construct
// in format strings — only raw content that came from outside the app.
func sanitizeContent(s string) string {
	return strings.ReplaceAll(s, "[", "[[]")
}

// renderMessages rebuilds the messageView from the committed buffer plus all
// active in-flight animation lines. Must always be called from the tview event loop.
func (c *ChatView) renderMessages() {
	text := c.committedText
	// Append in-flight lines in insertion order (IDs are sequential integers).
	for i := 0; i < c.nextAnimID; i++ {
		if line, ok := c.inFlight[i]; ok {
			text += line
		}
	}
	c.messageView.SetText(text)
	c.messageView.ScrollToEnd()
}

// ── Message formatting ────────────────────────────────────────────────────

// formatLine renders a Message into a tview-tagged string.
//
// Output format:   [HH:MM] [username] message body
//
// Both the username label (in brackets) and the message content share the
// same color so the entire line visually "belongs" to that user.
// [[] is tview's escape sequence for a literal "[" character.
func formatLine(msg *models.Message) string {
	if msg.IsSystem {
		return fmt.Sprintf("[yellow]▸ %s[-]\n", sanitizeContent(msg.Content))
	}
	color := msg.Color
	if color == "" {
		color = "[white]"
	}
	// sanitizeContent escapes [ in username and content so tview never
	// misinterprets user-supplied text as a color/style tag.
	return fmt.Sprintf("[dim][[]%s][-] %s[[]%s][-] %s%s[-]\n",
		msg.FormatTime(), color,
		sanitizeContent(msg.Username), color,
		sanitizeContent(msg.Content))
}

// incomingPrefix builds the formatted prefix for an incoming message line.
// Used by both static and animated rendering paths.
func incomingPrefix(colorTag, username string) string {
	return fmt.Sprintf("[dim][[]%s][-] %s[[]%s][-] %s",
		time.Now().Format("15:04"), colorTag, username, colorTag)
}

// ── Public message API ────────────────────────────────────────────────────

// AddMessage displays a message instantly (own messages, system messages).
// Must be called from the tview event loop.
//
// By appending to committedText (never to the raw messageView text), we
// guarantee the message survives any concurrent animation redraws.
func (c *ChatView) AddMessage(msg *models.Message) {
	c.committedText += formatLine(msg)
	c.renderMessages()
}

// AddIncomingMessage displays a message from another user.
//
//	colorTag — tview color tag from the wire format, e.g. "[green]" or "[#ff00ff]".
//	           Pass through models.ParseColorToTag if converting from raw JSON.
//
// Static mode  → appends to committedText immediately, one draw call.
// Anim mode    → allocates an in-flight slot, drips words via a goroutine.
//
// In both modes, any messages sent by the local user while this call is in
// progress are appended to committedText and will NOT be lost.
//
// Safe to call from any goroutine.
func (c *ChatView) AddIncomingMessage(username, content, colorTag string) {
	if atomic.LoadInt32(&c.stopped) == 1 {
		log.Printf("AddIncomingMessage: stopped, dropping msg from %s", username)
		return
	}

	// Normalise color tag
	if colorTag == "" {
		colorTag = models.GetUsernameColor(username)
	}
	if !strings.HasPrefix(colorTag, "[") {
		colorTag = models.ParseColorToTag(colorTag)
	}

	words := strings.Fields(content)
	if len(words) == 0 {
		return
	}

	prefix := incomingPrefix(colorTag, username)

	// ── STATIC mode ────────────────────────────────────────────────────────
	if atomic.LoadInt32(&c.animMode) == 0 {
		c.app.QueueUpdateDraw(func() {
			if atomic.LoadInt32(&c.stopped) == 1 {
				return
			}
			defer func() {
				if r := recover(); r != nil {
					log.Printf("PANIC static draw (from %s): %v", username, r)
				}
			}()
			c.committedText += prefix + sanitizeContent(content) + "[-]\n"
			c.renderMessages()
		})
		return
	}

	// ── ANIMATION mode ─────────────────────────────────────────────────────
	// Step 1 (event loop): allocate an in-flight slot and paint the cursor
	// immediately so the user sees activity straight away.
	// idCh carries both the animID and the inFlightGen at allocation time.
	// The animation goroutine uses gen to detect if ClearMessages() ran while
	// it was mid-flight, so it can discard stale word-tick callbacks.
	type animSlot struct{ id, gen int }
	slotCh := make(chan animSlot, 1)
	c.app.QueueUpdateDraw(func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("PANIC anim-init (from %s): %v", username, r)
				slotCh <- animSlot{-1, -1}
			}
		}()
		if atomic.LoadInt32(&c.stopped) == 1 {
			slotCh <- animSlot{-1, -1}
			return
		}
		animID := c.nextAnimID
		c.nextAnimID++
		gen := c.inFlightGen
		c.inFlight[animID] = prefix + "[dim]▋[-]"
		slotCh <- animSlot{animID, gen}
		c.renderMessages()
	})

	// Step 2 (goroutine): drip words one at a time, updating only our slot.
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("PANIC word-anim goroutine (from %s): %v", username, r)
			}
		}()

		slot := <-slotCh
		if slot.id < 0 || atomic.LoadInt32(&c.stopped) == 1 {
			return
		}
		animID := slot.id
		myGen := slot.gen

		built := ""
		for i, word := range words {
			if atomic.LoadInt32(&c.stopped) == 1 {
				return
			}

			// Variable delay: natural rhythm — short words fast, long ones slightly slower.
			delay := time.Duration(55+len(word)*9) * time.Millisecond
			if delay > 150*time.Millisecond {
				delay = 150 * time.Millisecond
			}
			time.Sleep(delay)

			if i == 0 {
				built = word
			} else {
				built += " " + word
			}
			isLast := i == len(words)-1
			snapshot := built

			c.app.QueueUpdateDraw(func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("PANIC word-anim draw (from %s): %v", username, r)
					}
				}()
				if atomic.LoadInt32(&c.stopped) == 1 {
					return
				}
				// If inFlightGen changed since we started, ClearMessages() ran.
				// Discard this callback — the map has been replaced and the
				// committed text has been wiped. Writing would be stale.
				if c.inFlightGen != myGen {
					return
				}
				if isLast {
					// Commit the finished line — remove from inFlight, append to committed.
					delete(c.inFlight, animID)
					c.committedText += prefix + sanitizeContent(snapshot) + "[-]\n"
				} else {
					// Still typing — update the in-flight slot only.
					// sanitizeContent ensures [ in content never triggers tview tag parsing.
					c.inFlight[animID] = prefix + sanitizeContent(snapshot) + " [dim]▋[-]"
				}
				c.renderMessages()
			})
		}
	}()
}

// SetMessages bulk-loads a slice of messages without animation.
// Replaces committedText entirely and clears any in-flight animations.
func (c *ChatView) SetMessages(messages []*models.Message) {
	if atomic.LoadInt32(&c.stopped) == 1 {
		return
	}
	c.app.QueueUpdateDraw(func() {
		if atomic.LoadInt32(&c.stopped) == 1 {
			return
		}
		var b strings.Builder
		for _, msg := range messages {
			b.WriteString(formatLine(msg))
		}
		c.committedText = b.String()
		c.inFlight = make(map[int]string) // discard any in-flight animations
		c.renderMessages()
	})
}

// ClearMessages wipes the message area and all in-flight animation state.
// Must be called from the tview event loop.
//
// Bumping inFlightGen invalidates any word-tick callbacks that were already
// queued when this runs — they check the generation and bail out rather than
// writing to a map that has been replaced.
func (c *ChatView) ClearMessages() {
	c.committedText = ""
	c.inFlight = make(map[int]string)
	c.inFlightGen++ // invalidate all queued animation callbacks
	c.renderMessages()
}

// ── Header ─────────────────────────────────────────────────────────────────

func (c *ChatView) startClockTicker() {
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if atomic.LoadInt32(&c.stopped) == 1 {
				return
			}
			c.app.QueueUpdateDraw(func() {
				if atomic.LoadInt32(&c.stopped) == 1 {
					return
				}
				c.redrawHeader()
			})
		}
	}()
}

// redrawHeader repaints the header content.
// Layout:  [GLOBAL]  HH:MM:SS  @username    ●ONLINE  LATENCY:Xms
// Must be called from within the tview event loop.
func (c *ChatView) redrawHeader() {
	clock := time.Now().Format("15:04:05")

	onlineStr := "[red]●OFFLINE[-]"
	if c.headerOnline {
		onlineStr = "[green]●ONLINE[-]"
	}

	userStr := ""
	if c.headerUsername != "" {
		userStr = fmt.Sprintf("  [yellow]@%s[-]", c.headerUsername)
	}

	latencyStr := "[dim]LATENCY:--ms[-]"
	if c.headerLatency >= 0 {
		latencyStr = fmt.Sprintf("[dim]LATENCY:%dms[-]", c.headerLatency)
	}

	c.header.SetText(fmt.Sprintf(
		"[cyan][GLOBAL][-]  [dim]%s[-]%s    %s  %s",
		clock, userStr, onlineStr, latencyStr,
	))
}

// SetCurrentUser pushes the logged-in username to the header.
// Must be called from the tview event loop.
func (c *ChatView) SetCurrentUser(username string) {
	c.headerUsername = username
	c.redrawHeader()
}

// SetOnlineStatus updates the ●ONLINE/●OFFLINE indicator in the header.
// Safe to call from any goroutine.
func (c *ChatView) SetOnlineStatus(online bool) {
	if atomic.LoadInt32(&c.stopped) == 1 {
		return
	}
	c.app.QueueUpdateDraw(func() {
		if atomic.LoadInt32(&c.stopped) == 1 {
			return
		}
		c.headerOnline = online
		c.redrawHeader()
	})
}

// UpdateLatency updates the latency shown in the header.
// Safe to call from any goroutine.
func (c *ChatView) UpdateLatency(latency int) {
	if atomic.LoadInt32(&c.stopped) == 1 {
		return
	}
	c.app.QueueUpdateDraw(func() {
		if atomic.LoadInt32(&c.stopped) == 1 {
			return
		}
		c.headerLatency = latency
		c.redrawHeader()
	})
}

// ── Command bar ───────────────────────────────────────────────────────────

func (c *ChatView) redrawCommandBar() {
	modeLabel := "[dim]mode:[green]ANIM[-]"
	if atomic.LoadInt32(&c.animMode) == 0 {
		modeLabel = "[dim]mode:[cyan]STATIC[-]"
	}
	nickLabel := ""
	if c.nickActive {
		nickLabel = "  [cyan]nick:ON ←→[-]"
	}
	c.commandBar.SetText(fmt.Sprintf(
		"[dim]/ commands: clear  whois  nick  mode  user_color  latency  info  exit  help[-]   %s%s",
		modeLabel, nickLabel,
	))
}

// ── Animation mode ────────────────────────────────────────────────────────

func (c *ChatView) SetAnimationMode(anim bool) {
	if anim {
		atomic.StoreInt32(&c.animMode, 1)
	} else {
		atomic.StoreInt32(&c.animMode, 0)
	}
	c.redrawCommandBar()
}

func (c *ChatView) ToggleAnimationMode() string {
	if atomic.LoadInt32(&c.animMode) == 1 {
		atomic.StoreInt32(&c.animMode, 0)
		c.redrawCommandBar()
		return "static"
	}
	atomic.StoreInt32(&c.animMode, 1)
	c.redrawCommandBar()
	return "animation"
}

func (c *ChatView) IsAnimationMode() bool {
	return atomic.LoadInt32(&c.animMode) == 1
}

// ── Nick mode ─────────────────────────────────────────────────────────────

func (c *ChatView) ToggleNickMode() bool {
	c.nickActive = !c.nickActive
	c.historyIdx = -1
	c.redrawCommandBar()
	return c.nickActive
}

func (c *ChatView) AddToHistory(msg string) {
	if msg == "" {
		return
	}
	if len(c.sentHistory) > 0 && c.sentHistory[len(c.sentHistory)-1] == msg {
		return
	}
	c.sentHistory = append(c.sentHistory, msg)
	if len(c.sentHistory) > 100 {
		c.sentHistory = c.sentHistory[1:]
	}
}

// ── Footer ────────────────────────────────────────────────────────────────

func (c *ChatView) UpdateCursorPosition(line, col int) {
	if atomic.LoadInt32(&c.stopped) == 1 {
		return
	}
	c.app.QueueUpdateDraw(func() {
		if atomic.LoadInt32(&c.stopped) == 1 {
			return
		}
		c.footer.SetText(fmt.Sprintf(
			"[magenta]NORMAL[-]    SecTherminal              UTF-8    L:%d, C:%d", line, col,
		))
	})
}

// Stop signals this view is permanently done. No further UI updates will run.
func (c *ChatView) Stop() {
	atomic.StoreInt32(&c.stopped, 1)
}
