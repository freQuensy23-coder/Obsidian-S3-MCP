package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"notesmcp/internal/auth"
	"notesmcp/internal/config"
	"notesmcp/internal/handler/mcp"
	"notesmcp/internal/storage/cache"
	"notesmcp/internal/storage/s3store"
	"notesmcp/internal/usecase/vault"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	serverCfg, err := config.LoadServerConfig()
	if err != nil {
		return err
	}

	envPath := os.Getenv("NOTES_MCP_ENV_FILE")
	if envPath == "" {
		envPath = ".env"
	}
	s3Cfg, err := config.LoadS3ConfigFile(envPath)
	if err != nil {
		return fmt.Errorf("load s3 config: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	store, err := s3store.New(ctx, s3Cfg)
	if err != nil {
		return fmt.Errorf("init s3 store: %w", err)
	}

	cachedStore := cache.New(store, 30*time.Second, time.Now)
	service := vault.NewService(cachedStore)
	handler := auth.BearerAuth(serverCfg.Token, mcp.NewHandler(service))

	mux := http.NewServeMux()
	mux.Handle("/mcp", handler)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	server := &http.Server{
		Addr:              serverCfg.Addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("notesmcp listening on http://%s/mcp", serverCfg.Addr)
	err = server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
