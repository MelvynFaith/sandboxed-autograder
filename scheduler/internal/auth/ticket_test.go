package auth

import (
	"errors"
	"testing"
	"time"
)

func TestTicketIsBoundAndSingleUse(t *testing.T) {
	store, err := NewTicketStore(MaxTicketTTL)
	if err != nil {
		t.Fatal(err)
	}
	raw, ticket, err := store.Issue(7, 91)
	if err != nil {
		t.Fatal(err)
	}
	if ticket.UserID != 7 || ticket.SubmissionID != 91 || time.Until(ticket.ExpiresAt) > MaxTicketTTL {
		t.Fatalf("ticket binding or expiry is incorrect: %+v", ticket)
	}
	consumed, err := store.Consume(raw)
	if err != nil || consumed != ticket {
		t.Fatalf("first Consume = %+v, %v; want %+v, nil", consumed, err, ticket)
	}
	if _, err := store.Consume(raw); !errors.Is(err, ErrInvalidTicket) {
		t.Fatalf("second Consume error = %v; want ErrInvalidTicket", err)
	}
}

func TestTicketRejectsExpiredValuesAndExcessiveLifetime(t *testing.T) {
	if _, err := NewTicketStore(MaxTicketTTL + time.Nanosecond); err == nil {
		t.Fatal("ticket store accepted a lifetime longer than 60 seconds")
	}
	store, err := NewTicketStore(time.Second)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	store.now = func() time.Time { return now }
	raw, _, err := store.Issue(1, 2)
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Second)
	if _, err := store.Consume(raw); !errors.Is(err, ErrInvalidTicket) {
		t.Fatalf("expired ticket error = %v; want ErrInvalidTicket", err)
	}
}
