package service

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAccountProxyAvailabilityControlsScheduling(t *testing.T) {
	now := time.Now().UTC()
	proxyID := int64(9)
	future := now.Add(time.Hour)
	past := now.Add(-time.Hour)

	tests := []struct {
		name  string
		proxy *Proxy
		state AccountProxyState
	}{
		{name: "active", proxy: &Proxy{ID: proxyID, Status: StatusActive, ExpiresAt: &future}, state: AccountProxyStateAvailable},
		{name: "expired by deadline", proxy: &Proxy{ID: proxyID, Status: StatusActive, ExpiresAt: &past}, state: AccountProxyStateExpired},
		{name: "expired by status", proxy: &Proxy{ID: proxyID, Status: StatusExpired}, state: AccountProxyStateExpired},
		{name: "disabled", proxy: &Proxy{ID: proxyID, Status: StatusDisabled}, state: AccountProxyStateUnavailable},
		{name: "missing", proxy: nil, state: AccountProxyStateUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := &Account{
				Status:      StatusActive,
				Schedulable: true,
				ProxyID:     &proxyID,
				Proxy:       tt.proxy,
			}
			require.Equal(t, tt.state, account.ProxyStateAt(now))
			// 与官方语义对齐：调度可调度性不再绑定代理状态（代理未加载/过期不阻止
			// 进入候选池；代理可用性由代理闸门与转发阶段判定）。
			require.True(t, account.IsSchedulableAt(now))
		})
	}
}

func TestValidateAccountProxyReturnsStableErrors(t *testing.T) {
	now := time.Now().UTC()
	proxyID := int64(9)
	past := now.Add(-time.Hour)

	expired := &Account{ProxyID: &proxyID, Proxy: &Proxy{ID: proxyID, Status: StatusActive, ExpiresAt: &past}}
	require.True(t, errors.Is(ValidateAccountProxy(expired, now), ErrAccountProxyExpired))

	unavailable := &Account{ProxyID: &proxyID, Proxy: &Proxy{ID: proxyID, Status: StatusDisabled}}
	require.True(t, errors.Is(ValidateAccountProxy(unavailable, now), ErrAccountProxyUnavailable))

	require.NoError(t, ValidateAccountProxy(&Account{}, now))
}
