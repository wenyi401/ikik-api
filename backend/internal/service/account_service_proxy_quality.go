package service

import (
	"context"
	"fmt"
	"time"

	"ikik-api/internal/pkg/httpclient"
)

const userPrivateProxyBaseProbeTimeout = 10 * time.Second

func (s *AccountService) CheckOwnedProxyQuality(ctx context.Context, ownerUserID, proxyID int64) (*ProxyQualityCheckResult, error) {
	if ownerUserID <= 0 {
		return nil, ErrUserNotFound
	}
	repo, err := s.userPrivateProxyRepo()
	if err != nil {
		return nil, err
	}
	proxy, err := repo.GetOwnedByID(ctx, ownerUserID, proxyID)
	if err != nil {
		return nil, err
	}
	return runPrivateProxyQualityCheck(ctx, proxy, s.proxyProber), nil
}

func runPrivateProxyConnectivityTest(ctx context.Context, proxy *Proxy, prober ProxyExitInfoProber) *ProxyTestResult {
	if proxy == nil {
		return &ProxyTestResult{Success: false, Message: "proxy not found"}
	}

	proxyURL := proxy.URL()
	var probeErr error
	if prober != nil {
		probeCtx, cancel := context.WithTimeout(ctx, userPrivateProxyBaseProbeTimeout)
		exitInfo, latencyMs, err := prober.ProbeProxy(probeCtx, proxyURL)
		cancel()
		if err == nil {
			return &ProxyTestResult{
				Success:     true,
				Message:     "Proxy is accessible",
				LatencyMs:   latencyMs,
				IPAddress:   exitInfo.IP,
				City:        exitInfo.City,
				Region:      exitInfo.Region,
				Country:     exitInfo.Country,
				CountryCode: exitInfo.CountryCode,
			}
		}
		probeErr = err
	}

	client, err := httpclient.GetClient(httpclient.Options{
		ProxyURL:              proxyURL,
		Timeout:               proxyQualityRequestTimeout,
		ResponseHeaderTimeout: proxyQualityResponseHeaderTimeout,
	})
	if err != nil {
		return &ProxyTestResult{Success: false, Message: fmt.Sprintf("create proxy test client: %v", err)}
	}

	targetCtx, cancel := context.WithTimeout(ctx, proxyQualityRequestTimeout)
	items := runProxyQualityTargets(targetCtx, client)
	cancel()
	for _, item := range items {
		if isProxyQualityTargetReachable(item.Status) {
			return &ProxyTestResult{
				Success:   true,
				Message:   fmt.Sprintf("%s is reachable: %s", item.Target, item.Message),
				LatencyMs: item.LatencyMs,
			}
		}
	}

	if len(items) > 0 && items[0].Message != "" {
		return &ProxyTestResult{Success: false, Message: fmt.Sprintf("AI targets unreachable: %s", items[0].Message)}
	}
	if probeErr != nil {
		return &ProxyTestResult{Success: false, Message: probeErr.Error()}
	}
	return &ProxyTestResult{Success: false, Message: "proxy test failed"}
}

func isProxyQualityTargetReachable(status string) bool {
	return status == "pass" || status == "warn" || status == "challenge"
}

func runPrivateProxyQualityCheck(ctx context.Context, proxy *Proxy, prober ProxyExitInfoProber) *ProxyQualityCheckResult {
	id := int64(0)
	if proxy != nil {
		id = proxy.ID
	}
	result := &ProxyQualityCheckResult{
		ProxyID:   id,
		Score:     100,
		Grade:     "A",
		CheckedAt: time.Now().Unix(),
		Items:     make([]ProxyQualityCheckItem, 0, len(proxyQualityTargets)+1),
	}
	if proxy == nil {
		result.Items = append(result.Items, ProxyQualityCheckItem{
			Target:  "base_connectivity",
			Status:  "fail",
			Message: "proxy not found",
		})
		result.FailedCount++
		finalizeProxyQualityResult(result)
		return result
	}

	proxyURL := proxy.URL()
	if prober == nil {
		result.Items = append(result.Items, ProxyQualityCheckItem{
			Target:  "base_connectivity",
			Status:  "warn",
			Message: "proxy probe service is unavailable",
		})
		result.WarnCount++
	} else {
		probeCtx, cancel := context.WithTimeout(ctx, userPrivateProxyBaseProbeTimeout)
		exitInfo, latencyMs, err := prober.ProbeProxy(probeCtx, proxyURL)
		cancel()
		if err != nil {
			result.Items = append(result.Items, ProxyQualityCheckItem{
				Target:    "base_connectivity",
				Status:    "warn",
				LatencyMs: latencyMs,
				Message:   fmt.Sprintf("exit info lookup failed; continuing AI reachability checks: %v", err),
			})
			result.WarnCount++
		} else {
			result.ExitIP = exitInfo.IP
			result.Country = exitInfo.Country
			result.CountryCode = exitInfo.CountryCode
			result.BaseLatencyMs = latencyMs
			result.Items = append(result.Items, ProxyQualityCheckItem{
				Target:    "base_connectivity",
				Status:    "pass",
				LatencyMs: latencyMs,
				Message:   "Proxy exit is reachable",
			})
			result.PassedCount++
		}
	}

	client, err := httpclient.GetClient(httpclient.Options{
		ProxyURL:              proxyURL,
		Timeout:               proxyQualityRequestTimeout,
		ResponseHeaderTimeout: proxyQualityResponseHeaderTimeout,
	})
	if err != nil {
		result.Items = append(result.Items, ProxyQualityCheckItem{
			Target:  "http_client",
			Status:  "fail",
			Message: fmt.Sprintf("create quality check client: %v", err),
		})
		result.FailedCount++
		finalizeProxyQualityResult(result)
		return result
	}

	for _, item := range runProxyQualityTargets(ctx, client) {
		result.Items = append(result.Items, item)
		applyProxyQualityItemCount(result, item)
	}

	finalizeProxyQualityResult(result)
	return result
}
