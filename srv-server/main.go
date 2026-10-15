package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/imamsyafii-dev/bbge-practice/srv-server/config"
	"github.com/imamsyafii-dev/bbge-practice/srv-server/db"
	accountapp "github.com/imamsyafii-dev/bbge-practice/srv-server/internal/app/account"
	accountdomain "github.com/imamsyafii-dev/bbge-practice/srv-server/internal/domain/account"
	"github.com/imamsyafii-dev/bbge-practice/srv-server/internal/router"
	"github.com/imamsyafii-dev/bbge-practice/srv-server/internal/store"
	"github.com/imamsyafii-dev/bbge-practice/srv-server/pkg/postgres"
)

const (
	httpReadHeaderTimeout = 5 * time.Second
	httpShutdownTimeout   = 10 * time.Second
)

type httpServerLifecycle interface {
	ListenAndServe() error
	Shutdown(context.Context) error
	Close() error
}

var _ httpServerLifecycle = (*http.Server)(nil)

func newHTTPServer(
	address string,
	handler http.Handler,
) *http.Server {
	return &http.Server{
		Addr:              address,
		Handler:           handler,
		ReadHeaderTimeout: httpReadHeaderTimeout,
	}
}

func runHTTPServer(
	appCtx context.Context,
	server httpServerLifecycle,
	onShutdownRequested func(),
) error {
	serverErrors := make(
		chan error,
		1,
	)

	go func() {
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if err == nil ||
			errors.Is(err, http.ErrServerClosed) {
			return nil
		}

		return fmt.Errorf(
			"listen and serve HTTP: %w",
			err,
		)

	case <-appCtx.Done():
		onShutdownRequested()
	}

	shutdownCtx, cancelShutdown := context.WithTimeout(
		context.Background(),
		httpShutdownTimeout,
	)

	shutdownErr := server.Shutdown(
		shutdownCtx,
	)
	cancelShutdown()

	var lifecycleErr error

	if shutdownErr != nil {
		lifecycleErr = fmt.Errorf(
			"shutdown HTTP server: %w",
			shutdownErr,
		)

		if closeErr := server.Close(); closeErr != nil {
			lifecycleErr = errors.Join(
				lifecycleErr,
				fmt.Errorf(
					"force close HTTP server: %w",
					closeErr,
				),
			)
		}
	}

	serveErr := <-serverErrors
	if serveErr != nil &&
		!errors.Is(
			serveErr,
			http.ErrServerClosed,
		) {
		lifecycleErr = errors.Join(
			lifecycleErr,
			fmt.Errorf(
				"listen and serve HTTP: %w",
				serveErr,
			),
		)
	}

	return lifecycleErr
}

func newAccountUseCase(
	accountStore accountdomain.Store,
) *accountapp.UseCase {
	accountService := accountdomain.NewService(
		accountStore,
	)

	return accountapp.NewUseCase(
		accountService,
	)
}

func newHTTPHandler(
	accountUseCase router.AccountUseCase,
) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc(
		"GET /health",
		healthHandler,
	)

	accountHandler := router.NewAccountHandler(
		accountUseCase,
	)
	accountHandler.Register(mux)

	return mux
}

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

	appCtx, stopSignals := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stopSignals()

	startupCtx, cancelStartup := context.WithTimeout(
		appCtx,
		5*time.Second,
	)

	database, err := postgres.Open(
		startupCtx,
		cfg.Postgres,
	)
	cancelStartup()

	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	log.Printf(
		"connected to PostgreSQL database %s",
		cfg.Postgres.Database,
	)

	if err := db.AutoMigrate(database); err != nil {
		log.Printf(
			"database migration failed: %v",
			err,
		)

		return
	}

	log.Printf("database migrations applied")

	queries := store.New(database)

	accountUseCase := newAccountUseCase(
		queries,
	)

	httpHandler := newHTTPHandler(
		accountUseCase,
	)

	server := newHTTPServer(
		cfg.HTTPAddr,
		httpHandler,
	)

	log.Printf(
		"practice server listening on http://localhost%s",
		server.Addr,
	)

	if err := runHTTPServer(
		appCtx,
		server,
		func() {
			log.Printf("shutdown signal received")
			stopSignals()
		},
	); err != nil {
		log.Printf(
			"practice server stopped: %v",
			err,
		)

		return
	}

	log.Printf("practice server stopped")
}
