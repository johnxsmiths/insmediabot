package resolver

import (
	"context"
	"fmt"
	"log"
	"time"

	"slmedia/pkg/config"
)

// Manager coordinates the provider fallback chain.
type Manager struct {
	providers map[string]Provider
	order     []string
}

// NewManager initializes the resolver manager with configured providers.
func NewManager(cfg *config.Config) *Manager {
	provMap := make(map[string]Provider)

	// Register all available real-world providers
	provMap["igexport"] = NewIGExportProvider(cfg.IGExportURL)
	provMap["fastdl"] = NewFastDLProvider("fastdl", cfg.FastDLURL, "")
	provMap["sssinstagram"] = NewFastDLProvider("sssinstagram", cfg.SSSInstagramURL, "")
	provMap["snapsave"] = NewSnapSaveProvider("snapsave", cfg.SnapSaveURL)
	provMap["saveclip"] = NewSnapSaveProvider("saveclip", cfg.SaveClipURL)
	provMap["savefromins"] = NewSaveFromInsProvider(cfg.SaveFromInsURL)

	return &Manager{
		providers: provMap,
		order:     cfg.ProviderOrder,
	}
}

// Resolve executes the fallback chain across configured providers until a valid media result is obtained.
func (m *Manager) Resolve(ctx context.Context, igURL string) (*MediaResult, error) {
	var lastErr error

	for _, name := range m.order {
		provider, exists := m.providers[name]
		if !exists {
			continue
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		provCtx, provCancel := context.WithTimeout(ctx, 4*time.Second)
		log.Printf("[Resolver] Trying provider: %s for %s", provider.Name(), igURL)
		result, err := provider.Resolve(provCtx, igURL)
		provCancel()

		if err == nil && result != nil && len(result.Items) > 0 {
			log.Printf("[Resolver] Success via provider: %s (found %d item(s))", provider.Name(), len(result.Items))
			return result, nil
		}

		if err != nil {
			log.Printf("[Resolver] Provider %s failed: %v", provider.Name(), err)
			lastErr = err
		}
	}

	if lastErr != nil {
		return nil, fmt.Errorf("all providers failed: %w", lastErr)
	}
	return nil, fmt.Errorf("no providers available or configured")
}
