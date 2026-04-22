package config

import (
	"context"
	"log/slog"
	"sync"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

const googleIssuer = "https://accounts.google.com"

// OIDCRuntime holds the live OIDC objects derived from discovery.
// Populated by InitOIDCProviders. May be nil if the provider is not configured.
type OIDCRuntime struct {
	Provider     *gooidc.Provider
	Verifier     *gooidc.IDTokenVerifier
	OAuth2Config oauth2.Config
}

// RuntimeProviders is populated at startup by InitOIDCProviders.
var RuntimeProviders struct {
	mu     sync.RWMutex
	OIDC   *OIDCRuntime
	Google *OIDCRuntime
}

// InitOIDCProviders performs OIDC discovery for configured providers.
// If a provider is unreachable it logs a warning and retries in the background.
func InitOIDCProviders(cfg *Config) {
	InitOIDCProvidersWithContext(context.Background(), cfg)
}

func InitOIDCProvidersWithContext(ctx context.Context, cfg *Config) {
	if cfg.OIDC.IssuerURL != "" {
		slog.Info("register OIDC auth provider", "url", cfg.OIDC.IssuerURL)
		go discoverWithRetry(ctx, "oidc", cfg.OIDC.IssuerURL, cfg.OIDC.ClientID, cfg.OIDC.ClientSecret,
			cfg.BaseURL.String()+"/auth/oidc/callback",
			func(rt *OIDCRuntime) {
				RuntimeProviders.mu.Lock()
				RuntimeProviders.OIDC = rt
				RuntimeProviders.mu.Unlock()
			})
	}
	if cfg.Google.ClientID != "" {
		slog.Info("register Google auth provider")
		go discoverWithRetry(ctx, "google", googleIssuer, cfg.Google.ClientID, cfg.Google.ClientSecret,
			cfg.BaseURL.String()+"/auth/google/callback",
			func(rt *OIDCRuntime) {
				RuntimeProviders.mu.Lock()
				RuntimeProviders.Google = rt
				RuntimeProviders.mu.Unlock()
			})
	}
}

// GetOIDCRuntime returns the current OIDC runtime (may be nil).
func GetOIDCRuntime() *OIDCRuntime {
	RuntimeProviders.mu.RLock()
	defer RuntimeProviders.mu.RUnlock()
	return RuntimeProviders.OIDC
}

// GetGoogleRuntime returns the current Google OIDC runtime (may be nil).
func GetGoogleRuntime() *OIDCRuntime {
	RuntimeProviders.mu.RLock()
	defer RuntimeProviders.mu.RUnlock()
	return RuntimeProviders.Google
}

func discoverWithRetry(ctx context.Context, name, issuerURL, clientID, clientSecret, redirectURL string, set func(*OIDCRuntime)) {
	for {
		rt, err := discover(ctx, issuerURL, clientID, clientSecret, redirectURL)
		if err != nil {
			slog.Warn("oidc discovery failed, retrying in 60s", "provider", name, "error", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(60 * time.Second):
			}
			continue
		}
		set(rt)
		slog.Info("oidc provider ready", "provider", name, "issuer", issuerURL)
		return
	}
}

func discover(ctx context.Context, issuerURL, clientID, clientSecret, redirectURL string) (*OIDCRuntime, error) {
	provider, err := gooidc.NewProvider(ctx, issuerURL)
	if err != nil {
		return nil, err
	}
	verifier := provider.Verifier(&gooidc.Config{ClientID: clientID})
	oauth2Cfg := oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{gooidc.ScopeOpenID, "email", "profile"},
	}
	return &OIDCRuntime{
		Provider:     provider,
		Verifier:     verifier,
		OAuth2Config: oauth2Cfg,
	}, nil
}
