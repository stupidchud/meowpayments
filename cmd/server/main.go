package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/meowpayments/meowpayments/internal/api"
	"github.com/meowpayments/meowpayments/internal/config"
	oneclickhttp "github.com/meowpayments/meowpayments/internal/http"
	"github.com/meowpayments/meowpayments/internal/hub"
	"github.com/meowpayments/meowpayments/internal/oneclick"
	"github.com/meowpayments/meowpayments/internal/store/postgres"
	"github.com/meowpayments/meowpayments/internal/worker"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(log)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// --- Config ---
	cfg := config.MustLoad()
	log.Info("meowpayments starting", "addr", cfg.HTTP.Addr)

	// --- Database ---
	pool := postgres.MustConnect(ctx, cfg.DB)
	defer pool.Close()

	if err := postgres.Migrate(cfg.DB.MigrationsDir, cfg.DB.DSN); err != nil {
		log.Error("migration failed", "err", err)
		os.Exit(1)
	}

	// --- Stores ---
	paymentStore := postgres.NewPaymentStore(pool)

	// --- 1-Click transport + client ---
	transport := oneclickhttp.NewTransport(cfg.OneClick.BaseURL, cfg.OneClick.APIKey, cfg.OneClick.Timeout)
	ocClient := oneclick.NewClient(transport)

	// Token cache (wraps the client - no circular dep because TokenCache calls Client)
	tokenCache := oneclick.NewTokenCache(ocClient, cfg.OneClick.TokenCacheTTL)

	// --- Hub ---
	h := hub.New()

	// --- Workers ---
	callbackSender := worker.NewCallbackSender(cfg.Worker, log)
	dispatcher := worker.NewDispatcher(h, callbackSender)
	poller := worker.NewPoller(paymentStore, ocClient, dispatcher, cfg.Poller, log)

	// --- Background goroutines ---
	go h.Run(ctx)
	go tokenCache.StartRefreshLoop(ctx)
	go poller.Run(ctx)
	go callbackSender.Run(ctx, cfg.Worker.CallbackWorkers)

	// --- HTTP server ---
	server := api.NewServer(&api.Dependencies{
		PaymentStore: paymentStore,
		OneClick:     ocClient,
		TokenCache:   tokenCache,
		Hub:          h,
		Cfg:          cfg,
	})

	log.Info("server ready", "addr", cfg.HTTP.Addr)
	if err := server.Run(); err != nil {
		log.Error("server exited", "err", err)
	}
}
