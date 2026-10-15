package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/imamsyafii-dev/bbge-practice/srv-server/config"
	"github.com/imamsyafii-dev/bbge-practice/srv-server/pkg/postgres"
)

func healthHandler(
	w http.ResponseWriter,
	_ *http.Request,
) {
	w.Header().Set(
		"Content-Type",
		"application/json",
	)
	w.WriteHeader(http.StatusOK)

	_, _ = w.Write(
		[]byte(`{"status":"ok"}`),
	)
}

func main() {
	cfg := config.Load()
	ctx := context.Background()

	database, err := postgres.Open(
		ctx,
		cfg.Postgres,
	)
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	log.Printf(
		"connected to PostgreSQL database %s",
		cfg.Postgres.Database,
	)

	mux := http.NewServeMux()

	mux.HandleFunc(
		"GET /health",
		healthHandler,
	)

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf(
		"practice server listening on http://localhost%s",
		server.Addr,
	)

	if err := server.ListenAndServe(); err != nil {
		log.Printf(
			"practice server stopped: %v",
			err,
		)
	}
}
