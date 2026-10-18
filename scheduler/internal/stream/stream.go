// Package stream serves the WebSocket that shows a student their submission being graded (FR-08,
// SRS 6.3): GET /ws/v1/submissions/{id}?ticket=TICKET.
//
// A browser cannot set an Authorization header on a WebSocket handshake, so the client first takes a
// one-time ticket from POST /api/v1/ws-tickets, which checks that the submission is the caller's
// (NFR-10), and presents it here. The ticket is consumed by the handshake whether or not it succeeds.
//
// Every frame is one JSON object, {"status", "current_test", "total_tests"} (progress.Event), each a
// full state rather than a change. The first frame is the state on connection; the server closes the
// connection with 1000 (normal closure) after a COMPLETED, FAILED or CANCELLED frame, and with 1001
// (going away) when it shuts down. The client sends nothing; pings keep the connection alive.
package stream

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/websocket"

	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/auth"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/progress"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/store"
)

const (
	// writeWait bounds one write to the client.
	writeWait = 10 * time.Second

	// pongWait is how long the client may stay silent; it answers a ping every pingPeriod.
	pongWait   = 60 * time.Second
	pingPeriod = pongWait * 9 / 10

	// maxMessage bounds what the client may send. It sends nothing but control frames.
	maxMessage = 512

	// closeGrace is how long to wait for the client to answer a close frame.
	closeGrace = time.Second
)

// Tickets consumes WebSocket tickets; *auth.TicketStore implements it.
type Tickets interface {
	Consume(value string) (auth.Ticket, error)
}

// States reads a submission's state from the database; *store.Pipeline implements it.
type States interface {
	Progress(ctx context.Context, submissionID int64) (progress.Event, error)
}

// Handler serves the stream. Mount it as "GET /ws/v1/submissions/{id}".
type Handler struct {
	tickets  Tickets
	states   States
	hub      *progress.Hub
	upgrader websocket.Upgrader
}

// New returns a Handler.
func New(tickets Tickets, states States, hub *progress.Hub) *Handler {
	return &Handler{
		tickets: tickets,
		states:  states,
		hub:     hub,
		upgrader: websocket.Upgrader{
			HandshakeTimeout: writeWait,
			// Any origin may connect. Cross-site WebSocket hijacking works by riding on the browser's
			// cookies, and this endpoint uses none: without a ticket, which only a request carrying the
			// user's access token can obtain, the handshake fails.
			CheckOrigin: func(*http.Request) bool { return true },
		},
	}
}

// ServeHTTP checks the ticket, upgrades the connection and streams the submission's progress.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !websocket.IsWebSocketUpgrade(r) {
		writeError(w, http.StatusBadRequest, "invalid_request", "Endpoint ini hanya menerima koneksi WebSocket.")
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusNotFound, "not_found", "Submission tidak ditemukan.")
		return
	}
	ticket, err := h.tickets.Consume(r.URL.Query().Get("ticket"))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid_ticket", "Tiket WebSocket tidak valid, kedaluwarsa, atau sudah dipakai.")
		return
	}
	// A ticket is bound to the submission it was issued for; another one is not the caller's to see.
	if ticket.SubmissionID != id {
		writeError(w, http.StatusNotFound, "not_found", "Submission tidak ditemukan.")
		return
	}

	// Subscribe before reading the database, so that nothing published in between is missed.
	sub := h.hub.Subscribe(id)
	defer sub.Close()
	state, err := h.states.Progress(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "Submission tidak ditemukan.")
		return
	}
	if err != nil {
		slog.Error("read submission progress", "submission", id, "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan pada server.")
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return // the upgrader has answered the client
	}
	defer conn.Close()
	stream(conn, sub, state, slog.With("submission", id, "user", ticket.UserID))
}

// stream sends the first state, then every event, until a terminal one, the client leaving, or the
// hub shutting down.
func stream(conn *websocket.Conn, sub *progress.Subscription, state progress.Event, log *slog.Logger) {
	// Read in the background: it answers pings and pongs and notices the client leaving.
	gone := make(chan struct{})
	go func() {
		defer close(gone)
		conn.SetReadLimit(maxMessage)
		_ = conn.SetReadDeadline(time.Now().Add(pongWait))
		conn.SetPongHandler(func(string) error { return conn.SetReadDeadline(time.Now().Add(pongWait)) })
		for {
			if _, _, err := conn.NextReader(); err != nil {
				return
			}
		}
	}()

	send := func(e progress.Event) bool {
		_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
		if err := conn.WriteJSON(e); err != nil {
			log.Debug("websocket write failed", "err", err)
			return false
		}
		return true
	}
	end := func(code int, text string) {
		msg := websocket.FormatCloseMessage(code, text)
		if conn.WriteControl(websocket.CloseMessage, msg, time.Now().Add(writeWait)) == nil {
			select {
			case <-gone:
			case <-time.After(closeGrace):
			}
		}
	}

	// A terminal state in the database is final. Otherwise the hub's latest event, if the
	// submission is being graded, is newer than what the database can say.
	if !state.Terminal() {
		if e, ok := sub.Next(); ok {
			state = e
		}
	}
	if !send(state) {
		return
	}
	if state.Terminal() {
		end(websocket.CloseNormalClosure, "")
		return
	}

	ping := time.NewTicker(pingPeriod)
	defer ping.Stop()
	for {
		select {
		case <-sub.Ready():
			e, ok := sub.Next()
			if !ok {
				continue
			}
			if !send(e) {
				return
			}
			if e.Terminal() {
				end(websocket.CloseNormalClosure, "")
				return
			}
		case <-ping.C:
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeWait)); err != nil {
				return
			}
		case <-sub.Done():
			end(websocket.CloseGoingAway, "server shutting down")
			return
		case <-gone:
			return
		}
	}
}

// writeError answers a request that was not upgraded, in the API's error format (SRS 6.3).
func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
}
