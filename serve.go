package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/SonyPradana/simpus-antrian-terminal/internal/config"
	"github.com/SonyPradana/simpus-antrian-terminal/internal/db"
	"github.com/SonyPradana/simpus-antrian-terminal/internal/httpapi"
	"github.com/SonyPradana/simpus-antrian-terminal/internal/service"
)

func runServe(args []string) error {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	configPath := flags.String("config", "", "path to config.json (optional)")
	addrOverride := flags.String("addr", "", "override HTTP_ADDR, for example :8080")
	dryRun := flags.Bool("dry-run", false, "validate only, without updating the database")

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() > 0 {
		return fmt.Errorf("unknown arguments: %v", flags.Args())
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	if strings.TrimSpace(*addrOverride) != "" {
		cfg.HTTPAddr = *addrOverride
	}
	if err := cfg.Validate(); err != nil {
		return err
	}

	sqlDB, err := db.Open(cfg)
	if err != nil {
		return err
	}
	defer func() { _ = sqlDB.Close() }()
	log.Printf("serve: connected to database %s:%d/%s", cfg.DBHost, cfg.DBPort, cfg.DBName)

	printerService := service.New(sqlDB)
	handler := httpapi.New(printerService, sqlDB, cfg.APIToken, *dryRun)
	httpServer := httpapi.NewHTTPServer(cfg, handler.Handler())

	errCh := make(chan error, 1)
	go func() {
		log.Printf("serve: listening on %s (dry-run=%t)", cfg.HTTPAddr, *dryRun)
		errCh <- httpServer.ListenAndServe()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("server stopped: %w", err)
		}
		return nil
	case <-sigCh:
		log.Printf("serve: received stop signal, shutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(ctx); err != nil {
			return fmt.Errorf("shutdown server: %w", err)
		}
		err := <-errCh
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("server stopped: %w", err)
		}
		return nil
	}
}
