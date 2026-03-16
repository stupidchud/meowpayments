package config

import (
	"time"

	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	HTTP     HTTPConfig
	DB       DBConfig
	Auth     AuthConfig
	OneClick OneClickConfig
	Poller   PollerConfig
	Worker   WorkerConfig
}

type HTTPConfig struct {
	Addr           string        `envconfig:"HTTP_ADDR"            default:":8080"`
	BaseURL        string        `envconfig:"HTTP_BASE_URL"        required:"true"`
	AllowedOrigins string        `envconfig:"HTTP_ALLOWED_ORIGINS" default:"*"`
	ReadTimeout    time.Duration `envconfig:"HTTP_READ_TIMEOUT"    default:"30s"`
	WriteTimeout   time.Duration `envconfig:"HTTP_WRITE_TIMEOUT"   default:"60s"`
}

type DBConfig struct {
	DSN           string `envconfig:"DATABASE_URL" required:"true"`
	MaxConns      int32  `envconfig:"DB_MAX_CONNS" default:"20"`
	MinConns      int32  `envconfig:"DB_MIN_CONNS" default:"2"`
	MigrationsDir string `envconfig:"DB_MIGRATIONS_DIR" default:"./migrations"`
}

type AuthConfig struct {
	// APIKey is the single secret key the operator uses to authenticate with the API.
	// Set a strong random value (e.g. openssl rand -hex 32).
	APIKey string `envconfig:"API_KEY" required:"true"`
}

type OneClickConfig struct {
	BaseURL            string        `envconfig:"ONECLICK_BASE_URL"          default:"https://1click.chaindefuser.com"`
	APIKey             string        `envconfig:"ONECLICK_API_KEY"           required:"true"`
	Timeout            time.Duration `envconfig:"ONECLICK_TIMEOUT"           default:"30s"`
	TokenCacheTTL      time.Duration `envconfig:"ONECLICK_TOKEN_CACHE_TTL"   default:"5m"`
	DefaultSlippageBPS int           `envconfig:"ONECLICK_DEFAULT_SLIPPAGE"  default:"100"`
	// RefundAddress is used as the refundTo address for all quotes.
	// Since we use ANY_INPUT we don't know the customer source chain at quote time.
	// Configure this to a NEAR account or well-known address you control.
	RefundAddress string `envconfig:"ONECLICK_REFUND_ADDRESS" required:"true"`
}

type PollerConfig struct {
	FastInterval    time.Duration `envconfig:"POLLER_FAST_INTERVAL"    default:"10s"`
	SlowInterval    time.Duration `envconfig:"POLLER_SLOW_INTERVAL"    default:"30s"`
	MaxPollFailures int           `envconfig:"POLLER_MAX_FAILURES"     default:"10"`
}

type WorkerConfig struct {
	CallbackWorkers    int           `envconfig:"WORKER_CALLBACK_WORKERS"     default:"5"`
	CallbackMaxRetries int           `envconfig:"WORKER_CALLBACK_MAX_RETRIES" default:"5"`
	CallbackTimeout    time.Duration `envconfig:"WORKER_CALLBACK_TIMEOUT"     default:"10s"`
	// WebhookSecret is used to sign HMAC-SHA256 callback payloads.
	WebhookSecret string `envconfig:"WORKER_WEBHOOK_SECRET" required:"true"`
}

// MustLoad loads config from environment variables, panicking on error.
func MustLoad() *Config {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		panic("config: " + err.Error())
	}
	return &cfg
}
