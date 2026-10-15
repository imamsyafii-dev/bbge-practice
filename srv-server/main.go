package main

import (
	"log"
	"net/http"
	"time"

	"github.com/imamsyafii-dev/bbge-practice/srv-server/config"
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
		log.Fatal(err)
	}
}
