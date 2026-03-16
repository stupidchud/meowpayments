// Package api wires the Fuego HTTP server with all routes and middleware.
package api

import (
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/go-fuego/fuego"
	"github.com/meowpayments/meowpayments/internal/api/handlers"
	apimw "github.com/meowpayments/meowpayments/internal/api/middleware"
	"github.com/meowpayments/meowpayments/internal/auth"
	"github.com/meowpayments/meowpayments/internal/config"
	"github.com/meowpayments/meowpayments/internal/hub"
	"github.com/meowpayments/meowpayments/internal/oneclick"
	"github.com/meowpayments/meowpayments/internal/store"
)

// Dependencies groups all injected services for the API server.
type Dependencies struct {
	PaymentStore store.PaymentStore
	OneClick     oneclick.Client
	TokenCache   *oneclick.TokenCache
	Hub          *hub.Hub
	Cfg          *config.Config
}

// NewServer creates and configures the Fuego application.
func NewServer(deps *Dependencies) *fuego.Server {
	cfg := deps.Cfg

	s := fuego.NewServer(
		fuego.WithAddr(cfg.HTTP.Addr),
		fuego.WithEngineOptions(
			fuego.WithOpenAPIConfig(fuego.OpenAPIConfig{
				PrettyFormatJSON:     true,
				DisableDefaultServer: true,
			}),
		),
	)

	s.OpenAPI.Description().AddServer(&openapi3.Server{URL: cfg.HTTP.BaseURL})

	// Global middleware
	fuego.Use(s, apimw.CORS(cfg.HTTP.AllowedOrigins))

	// Handler groups
	payments := handlers.NewPaymentHandlers(deps.PaymentStore, deps.OneClick, deps.TokenCache, cfg)
	tokens := handlers.NewTokenHandlers(deps.TokenCache)
	ws := handlers.NewWSHandlers(deps.Hub, deps.PaymentStore)

	// Public routes (no auth)

	fuego.Get(s, "/v1/health", handlers.Health)
	fuego.Get(s, "/v1/tokens", tokens.List)

	// Customer-facing payment page endpoints
	fuego.Get(s, "/v1/pay/{id}", payments.GetPublic)
	fuego.Post(s, "/v1/pay/{id}/submit", payments.SubmitDeposit)
	fuego.Get(s, "/v1/pay/{id}/ws", ws.Payment)

	// Operator routes (require API key)

	authMW := auth.Middleware(cfg.Auth.APIKey)

	protected := fuego.Group(s, "/v1")
	fuego.Use(protected, authMW)

	fuego.Post(protected, "/payments", payments.Create)
	fuego.Get(protected, "/payments", payments.List)
	fuego.Get(protected, "/payments/{id}", payments.Get)
	fuego.Delete(protected, "/payments/{id}", payments.Cancel)
	fuego.Get(protected, "/payments/{id}/events", payments.GetEvents)

	// Operator global WebSocket - also accepts ?token= for browser clients
	fuego.Get(protected, "/ws", ws.Global)

	return s
}
