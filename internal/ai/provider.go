// Package ai defines the unified interface for AI image providers.
package ai

import (
	"context"
	"errors"
	"log/slog"
	"time"

	cache "github.com/go-pkgz/expirable-cache/v3"
)

const (
	modelCacheTTL     = 5 * time.Minute
	modelCacheMaxKeys = 512
)

//nolint:gochecknoglobals // shared TTL cache for provider model lists, safe concurrent access
var modelCache = cache.NewCache[string, []Model]().
	WithMaxKeys(modelCacheMaxKeys).
	WithTTL(modelCacheTTL)

// ProviderID identifies an AI provider.
type ProviderID string

// KeySource indicates where a provider's API key was configured (e.g. workspace, env, none).
type KeySource string

type KeyStatus struct {
	KeySet bool      `json:"key_set"`
	Source KeySource `json:"source"`
}

var (
	ErrNotCapable        = errors.New("ai: this api provider cannot do that")
	ErrInvalidModel      = errors.New("ai: model not found or invalid")
	ErrNotConfigured     = errors.New("ai: api key not configured")
	ErrUnknownProvider   = errors.New("ai: unknown provider")
	ErrAPIError          = errors.New("ai: api returned non-2xx")
	ErrInvalidKey        = errors.New("ai: invalid api key")
	ErrModelNotSupported = errors.New("ai: model does not support this operation")
)

const (
	ProviderImageRouter ProviderID = "imagerouter"
	ProviderOpenRouter  ProviderID = "openrouter"

	SourceWorkspace KeySource = "workspace"
	SourceNone      KeySource = "none"
	SourceEnv       KeySource = "env"
)

// Capability is a bitmask of features a provider may support.
type Capability uint32

var capNames = map[Capability]string{
	CapBgRemove:         "background removal",
	CapImageToImage:     "image to image",
	CapTextToImage:      "text to image",
	CapImageToText:      "image to text",
	CapImageDescription: "image description",
}

func (c Capability) Names() []string {
	var names []string
	for i, name := range capNames {
		if c&i != 0 {
			names = append(names, name)
		}
	}
	return names
}

const (
	CapBgRemove         Capability = 1 << iota // image → image, background removal
	CapImageToImage                            // image + prompt → image
	CapTextToImage                             // prompt → image (reserved)
	CapImageToText                             // image → text (reserved)
	CapImageDescription                        // image → text description via vision model
)

// Provider is a configured, ready-to-use AI provider.
// Only methods matching the provider's declared Capabilities are safe to call.
type Provider interface {
	ID() ProviderID
	Capabilities() Capability
	IsConfigured() bool
	KeySource() KeySource
	// BgRemove removes the background from imageData. Returns PNG bytes.
	BgRemove(ctx context.Context, imageData []byte, model string) ([]byte, error)
	// Transform applies a prompt-guided transform. Returns PNG bytes.
	Transform(ctx context.Context, imageData []byte, prompt, model string) ([]byte, error)
	// ListModels returns models available for this provider.
	ListModels(ctx context.Context) ([]Model, error)
	// ValidateKey checks that the configured API key is accepted by the provider.
	// Returns ErrInvalidKey if rejected, or another error for transport failures.
	ValidateKey(ctx context.Context) error
}

type ProviderWithModels struct {
	Provider

	Models []Model
}

// Model is a provider model returned to clients.
type Model struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	ProviderID    ProviderID `json:"provider_id"`
	PricePerImage float64    `json:"price_per_image"`
	Capabilities  Capability `json:"capability"`
}

// ProviderFactory constructs a Provider from resolved credentials.
// The default implementation is NewProvider.
type ProviderFactory func(providerID ProviderID, apiKey string, keySource KeySource) (Provider, error)

// ResetModelCacheForTest clears the package-level model cache. Call in tests to prevent cross-test pollution.
func ResetModelCacheForTest() {
	modelCache.Purge()
}

func NewProvider(providerID ProviderID, apiKey string, keySource KeySource) (Provider, error) {
	switch providerID {
	case ProviderImageRouter:
		return NewImageRouterProvider(apiKey, keySource, true), nil
	case ProviderOpenRouter:
		return NewOpenRouterProvider(
			apiKey,
			keySource,
			"google/gemini-2.5-flash:free",
			"google/gemini-2.5-flash:free",
		), nil
	default:
		return nil, ErrUnknownProvider
	}
}

func AllProviders(
	ctx context.Context,
	workspaceID string,
	apiKeyResolver KeyResolver,
	capabilities Capability,
) ([]ProviderWithModels, error) {
	pkIDs := []ProviderID{ProviderImageRouter, ProviderOpenRouter}
	pks := []ProviderWithModels{}

	for _, pk := range pkIDs {
		apiKey, source, keyErr := apiKeyResolver(ctx, workspaceID, string(pk))
		if keyErr != nil {
			return pks, keyErr
		}
		p, pErr := NewProvider(pk, apiKey, source)
		if pErr != nil {
			return pks, pErr
		}
		pm := ProviderWithModels{
			Provider: p,
			Models:   []Model{},
		}
		if p.IsConfigured() {
			models, lErr := p.ListModels(ctx)
			if lErr != nil {
				slog.WarnContext(ctx, "ai: list models failed", "provider", string(pk), "error", lErr)
			}
			for _, m := range models {
				if (m.Capabilities & capabilities) == 0 {
					continue
				}
				pm.Models = append(pm.Models, m)
			}
		}
		pks = append(pks, pm)
	}

	return pks, nil
}
