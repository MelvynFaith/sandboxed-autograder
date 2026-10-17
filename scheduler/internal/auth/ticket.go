package auth

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"sync"
	"time"
)

const MaxTicketTTL = 60 * time.Second

var ErrInvalidTicket = errors.New("invalid, expired, or already consumed WebSocket ticket")

type Ticket struct {
	UserID       int64
	SubmissionID int64
	ExpiresAt    time.Time
}

type ticketEntry struct {
	ticket Ticket
}

// TicketStore keeps short-lived, single-use tickets in process memory.
type TicketStore struct {
	mu      sync.Mutex
	ttl     time.Duration
	now     func() time.Time
	entries map[string]ticketEntry
}

func NewTicketStore(ttl time.Duration) (*TicketStore, error) {
	if ttl <= 0 || ttl > MaxTicketTTL {
		return nil, errors.New("WebSocket ticket lifetime must be between 1 and 60 seconds")
	}
	return &TicketStore{
		ttl:     ttl,
		now:     time.Now,
		entries: make(map[string]ticketEntry),
	}, nil
}

func (s *TicketStore) Issue(userID, submissionID int64) (string, Ticket, error) {
	if userID <= 0 || submissionID <= 0 {
		return "", Ticket{}, errors.New("ticket requires positive user and submission IDs")
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", Ticket{}, err
	}
	value := base64.RawURLEncoding.EncodeToString(raw)
	ticket := Ticket{
		UserID:       userID,
		SubmissionID: submissionID,
		ExpiresAt:    s.now().Add(s.ttl),
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.removeExpiredLocked(s.now())
	s.entries[value] = ticketEntry{ticket: ticket}
	return value, ticket, nil
}

func (s *TicketStore) Consume(value string) (Ticket, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.entries[value]
	if !ok {
		return Ticket{}, ErrInvalidTicket
	}
	delete(s.entries, value)
	if !s.now().Before(entry.ticket.ExpiresAt) {
		return Ticket{}, ErrInvalidTicket
	}
	return entry.ticket, nil
}

func (s *TicketStore) removeExpiredLocked(now time.Time) {
	for key, entry := range s.entries {
		if !now.Before(entry.ticket.ExpiresAt) {
			delete(s.entries, key)
		}
	}
}
