package service

import (
	"context"
	"errors"
	"net"
	"testing"
)

func TestClassifyOpenAITransportError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		persistent bool
	}{
		{
			name:       "socks auth failure",
			err:        errors.New("socks connect tcp 127.0.0.1:1080->example.com:443: username/password authentication failed"),
			persistent: true,
		},
		{
			name:       "dns not found",
			err:        &net.DNSError{Err: "no such host", Name: "bad.example", IsNotFound: true},
			persistent: true,
		},
		{
			name:       "temporary timeout",
			err:        context.DeadlineExceeded,
			persistent: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyOpenAITransportError(tt.err)
			if got.Persistent != tt.persistent {
				t.Fatalf("Persistent = %v, want %v", got.Persistent, tt.persistent)
			}
		})
	}
}
