package service

import (
	"context"
	"net/http"
	"sync"
)

func runProxyQualityTargets(ctx context.Context, client *http.Client) []ProxyQualityCheckItem {
	items := make([]ProxyQualityCheckItem, len(proxyQualityTargets))
	var wg sync.WaitGroup
	for index, target := range proxyQualityTargets {
		index, target := index, target
		wg.Add(1)
		go func() {
			defer wg.Done()
			items[index] = runProxyQualityTarget(ctx, client, target)
		}()
	}
	wg.Wait()
	return items
}

func applyProxyQualityItemCount(result *ProxyQualityCheckResult, item ProxyQualityCheckItem) {
	if result == nil {
		return
	}
	switch item.Status {
	case "pass":
		result.PassedCount++
	case "warn":
		result.WarnCount++
	case "challenge":
		result.ChallengeCount++
	default:
		result.FailedCount++
	}
}
