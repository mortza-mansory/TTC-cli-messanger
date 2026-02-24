package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rivo/tview"
)

// DefaultServerURL is the ONLY server this client will ever talk to.
// Internet reachability is NOT checked — if this host is down the app exits.
var DefaultServerURL = "http://localhost:8034"

// serverAccessKey must match the backend's configured key exactly.
const serverAccessKey = "secure_chat_key_2024"

// ── Wire types — matching the backend API exactly ─────────────────────────────

// sendRequest mirrors POST /api/send body.
type sendRequest struct {
	AccessKey string `json:"access_key"`
	ClientID  string `json:"client_id"`
	Username  string `json:"username"`
	Content   string `json:"content"`
	Color     string `json:"color"`
}

// sendResponse mirrors the POST /api/send success response.
type sendResponse struct {
	Status string `json:"status"`
	ID     string `json:"id"`
	Time   string `json:"time"`
}

// pollMessage is one entry from the GET /api/poll array.
// The backend uses the username as the message-content key, e.g.:
//
//	{ "script_kiddie": "Anyone using Go 1.22?", "color": "[yellow]", "id": "...", "timestamp": "..." }
//
// We parse with a raw map and extract the dynamic username key.
type pollMessage struct {
	Username  string
	Content   string
	Color     string
	ID        string
	Timestamp time.Time
}

// knownPollKeys lists all fixed keys in a poll message object.
// Every other key is treated as the username.
var knownPollKeys = map[string]bool{
	"color":     true,
	"id":        true,
	"timestamp": true,
}

// parsePollMessages parses the raw JSON array from /api/poll.
// Each element has a dynamic username key alongside fixed metadata keys.
func parsePollMessages(data []byte) ([]*pollMessage, error) {
	var rawList []map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawList); err != nil {
		return nil, fmt.Errorf("parse poll array: %w", err)
	}

	msgs := make([]*pollMessage, 0, len(rawList))
	for _, raw := range rawList {
		msg := &pollMessage{}

		// Fixed fields
		if v, ok := raw["color"]; ok {
			json.Unmarshal(v, &msg.Color)
		}
		if v, ok := raw["id"]; ok {
			json.Unmarshal(v, &msg.ID)
		}
		if v, ok := raw["timestamp"]; ok {
			json.Unmarshal(v, &msg.Timestamp)
		}

		// Dynamic field: the one key that is NOT in knownPollKeys is the username,
		// and its string value is the message content.
		for key, val := range raw {
			if knownPollKeys[key] {
				continue
			}
			msg.Username = key
			json.Unmarshal(val, &msg.Content)
			break
		}

		if msg.Username == "" || msg.Content == "" || msg.ID == "" {
			log.Printf("NetworkClient: skipping malformed poll entry (id=%s user=%s)", msg.ID, msg.Username)
			continue
		}
		msgs = append(msgs, msg)
	}
	return msgs, nil
}

// ── NetworkClient ──────────────────────────────────────────────────────────────

// NetworkClient handles all HTTP communication with the SecTherminal relay server.
//
// Concurrency:
//   - SendMessage is safe from any goroutine (runs in its own goroutine).
//   - pollLoop runs in a dedicated goroutine started by Start().
//   - onMessage / onStatusChange are called from those goroutines and must
//     schedule UI updates via app.QueueUpdateDraw themselves.
type NetworkClient struct {
	serverURL string
	clientID  string // unique per session, sent with every request
	app       *tview.Application

	httpClient *http.Client
	stopped    int32 // atomic: 1 = shut down
	stopCh     chan struct{}

	lastIDMu sync.Mutex
	lastID   string // cursor for incremental polling

	sentIDsMu sync.Mutex
	sentIDs   map[string]struct{} // IDs of our own sent messages (to skip echo)

	onMessage      func(username, content, colorTag string)
	onStatusChange func(connected bool, msg string)
}

// NewNetworkClient creates a NetworkClient ready to Start().
func NewNetworkClient(
	app *tview.Application,
	serverURL string,
	onMessage func(username, content, colorTag string),
	onStatusChange func(connected bool, msg string),
) *NetworkClient {
	return &NetworkClient{
		serverURL: serverURL,
		clientID:  generateClientID(),
		app:       app,
		// Timeout must exceed the server's long-poll window.
		// Backend holds requests for up to 30s → we use 40s.
		httpClient:     &http.Client{Timeout: 40 * time.Second},
		stopCh:         make(chan struct{}),
		sentIDs:        make(map[string]struct{}),
		onMessage:      onMessage,
		onStatusChange: onStatusChange,
	}
}

// generateClientID produces a random session identifier.
func generateClientID() string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return fmt.Sprintf("client_%d", r.Int63n(1_000_000_000))
}

// ── Public API ─────────────────────────────────────────────────────────────────

// Start begins the long-polling receive loop. Call Stop() to shut it down.
func (nc *NetworkClient) Start() {
	go nc.pollLoop()
}

// SendMessage POSTs a message to the server asynchronously.
// The caller is responsible for displaying the message locally before calling this.
func (nc *NetworkClient) SendMessage(username, content, colorTag string) {
	if atomic.LoadInt32(&nc.stopped) == 1 {
		return
	}
	go nc.sendAsync(username, content, colorTag)
}

// Stop shuts down the client cleanly. Idempotent.
func (nc *NetworkClient) Stop() {
	if atomic.CompareAndSwapInt32(&nc.stopped, 0, 1) {
		close(nc.stopCh)
	}
}

// ── Send ───────────────────────────────────────────────────────────────────────

func (nc *NetworkClient) sendAsync(username, content, colorTag string) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("NetworkClient.sendAsync panic: %v", r)
		}
	}()

	body := sendRequest{
		AccessKey: serverAccessKey,
		ClientID:  nc.clientID,
		Username:  username,
		Content:   content,
		Color:     colorTag,
	}
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		log.Printf("NetworkClient: marshal send: %v", err)
		return
	}

	resp, err := nc.httpClient.Post(
		nc.serverURL+"/api/send",
		"application/json",
		bytes.NewReader(bodyJSON),
	)
	if err != nil {
		log.Printf("NetworkClient: POST /api/send: %v", err)
		nc.notifyStatus(false, "Message send failed — server unreachable.")
		return
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		nc.notifyStatus(false, "Server rejected access key.")
		return
	case http.StatusOK, http.StatusCreated:
		var sr sendResponse
		if err := json.NewDecoder(resp.Body).Decode(&sr); err == nil && sr.ID != "" {
			// Register the message ID so the poll loop skips the server's echo.
			nc.sentIDsMu.Lock()
			nc.sentIDs[sr.ID] = struct{}{}
			nc.sentIDsMu.Unlock()
		}
	default:
		body, _ := io.ReadAll(resp.Body)
		log.Printf("NetworkClient: send HTTP %d: %.120s", resp.StatusCode, body)
		nc.notifyStatus(false, fmt.Sprintf("Send error (HTTP %d).", resp.StatusCode))
	}
}

// ── Receive (long poll) ────────────────────────────────────────────────────────

func (nc *NetworkClient) pollLoop() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("NetworkClient.pollLoop panic: %v", r)
		}
	}()

	backoff := 1 * time.Second
	const maxBackoff = 30 * time.Second
	firstConnect := true
	wasConnected := false

	for {
		if atomic.LoadInt32(&nc.stopped) == 1 {
			return
		}

		msgs, err := nc.poll()
		if err != nil {
			log.Printf("NetworkClient: poll: %v", err)
			if firstConnect {
				nc.notifyStatus(false, fmt.Sprintf(
					"Cannot reach server at %s", nc.serverURL))
			} else if wasConnected {
				nc.notifyStatus(false, fmt.Sprintf(
					"Connection lost — reconnecting in %v…", backoff))
			}
			wasConnected = false

			select {
			case <-nc.stopCh:
				return
			case <-time.After(backoff):
			}
			backoff = min(backoff*2, maxBackoff)
			continue
		}

		// Successful poll.
		if firstConnect || !wasConnected {
			nc.notifyStatus(true, fmt.Sprintf("Connected to relay at %s", nc.serverURL))
		}
		backoff = 1 * time.Second
		firstConnect = false
		wasConnected = true

		for _, msg := range msgs {
			nc.handleIncoming(msg)
		}

		// 204 No Content means no new messages; brief pause before next poll.
		if msgs == nil {
			select {
			case <-nc.stopCh:
				return
			case <-time.After(500 * time.Millisecond):
			}
		}
	}
}

// poll performs one GET /api/poll.
// Returns (nil, nil) on 204 No Content (nothing new).
// Returns ([]*pollMessage, nil) on success.
// Returns (nil, error) on any failure.
func (nc *NetworkClient) poll() ([]*pollMessage, error) {
	nc.lastIDMu.Lock()
	lastID := nc.lastID
	nc.lastIDMu.Unlock()

	params := url.Values{}
	params.Set("access_key", serverAccessKey)
	params.Set("client_id", nc.clientID)
	if lastID != "" {
		params.Set("last_id", lastID)
	}

	req, err := http.NewRequest(http.MethodGet,
		nc.serverURL+"/api/poll?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := nc.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNoContent:
		return nil, nil // no new messages

	case http.StatusUnauthorized:
		return nil, fmt.Errorf("server rejected access key")

	case http.StatusOK:
		rawBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("read poll body: %w", err)
		}
		msgs, err := parsePollMessages(rawBody)
		if err != nil {
			return nil, err
		}
		if len(msgs) > 0 {
			nc.lastIDMu.Lock()
			nc.lastID = msgs[len(msgs)-1].ID
			nc.lastIDMu.Unlock()
		}
		return msgs, nil

	default:
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected HTTP %d: %.120s", resp.StatusCode, body)
	}
}

// handleIncoming dispatches a received message, skipping our own echoed messages.
func (nc *NetworkClient) handleIncoming(msg *pollMessage) {
	nc.sentIDsMu.Lock()
	_, isMine := nc.sentIDs[msg.ID]
	if isMine {
		delete(nc.sentIDs, msg.ID)
	}
	nc.sentIDsMu.Unlock()
	if isMine {
		return
	}

	if nc.onMessage != nil {
		nc.onMessage(msg.Username, msg.Content, msg.Color)
	}
}

func (nc *NetworkClient) notifyStatus(connected bool, msg string) {
	if nc.onStatusChange != nil {
		nc.onStatusChange(connected, msg)
	}
}

// ── Startup connectivity check ─────────────────────────────────────────────────

// CheckServerConnectivity probes GET /health on DefaultServerURL with a 3-second
// timeout. This intentionally does NOT check general internet access — if the
// backend at DefaultServerURL is unreachable the application must exit, regardless
// of whether the user has internet connectivity.
func CheckServerConnectivity(serverURL string) error {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(serverURL + "/health")
	if err != nil {
		return fmt.Errorf("relay server not available at %s: %w", serverURL, err)
	}
	resp.Body.Close()
	if resp.StatusCode >= 500 {
		return fmt.Errorf("relay server returned HTTP %d — server error", resp.StatusCode)
	}
	return nil
}

func min(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
