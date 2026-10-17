package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/api"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/auth"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	databaseURL := os.Getenv("DATABASE_URL")
	jwtSecret := os.Getenv("JWT_SECRET")
	address := os.Getenv("HTTP_ADDR")
	if databaseURL == "" || jwtSecret == "" || address == "" {
		return errors.New("DATABASE_URL, JWT_SECRET, and HTTP_ADDR environment variables are required")
	}
	tokens, err := auth.NewTokens([]byte(jwtSecret))
	if err != nil {
		return err
	}
	ticketStore, err := auth.NewTicketStore(auth.MaxTicketTTL)
	if err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return err
	}

	server := &http.Server{
		Addr:              address,
		Handler:           api.NewHandler(auth.NewStore(pool), tokens, ticketStore),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	serverErrors := make(chan error, 1)
	go func() {
		log.Printf("scheduler API listening on %s", address)
		serverErrors <- server.ListenAndServe()
	}()
	select {
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		return server.Shutdown(shutdownCtx)
	}
}
