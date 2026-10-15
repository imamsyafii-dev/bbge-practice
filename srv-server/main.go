package main

import (
	"log"
	"net/http"
	"time"
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
	mux := http.NewServeMux()

	mux.HandleFunc(
		"GET /health",
		healthHandler,
	)

	server := &http.Server{
		Addr:              ":18080",
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
