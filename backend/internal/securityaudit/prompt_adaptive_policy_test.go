package securityaudit

import (
	"fmt"
	"testing"
)

func TestShouldRunRemotePromptAuditPolicy(t *testing.T) {
	tests := []struct {
		name         string
		forceRemote  bool
		remoteAudits int64
		sampleRate   int
		want         bool
	}{
		{name: "first audit", remoteAudits: 0, sampleRate: 0, want: true},
		{name: "hundredth audit", remoteAudits: 99, sampleRate: 0, want: true},
		{name: "trusted sampled out", remoteAudits: 100, sampleRate: 0, want: false},
		{name: "structural signal always reviewed", forceRemote: true, remoteAudits: 100, sampleRate: 0, want: true},
		{name: "watch profile always reviewed", remoteAudits: 100, sampleRate: 100, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldRunRemotePromptAudit(tt.forceRemote, tt.remoteAudits, 42, "request", 101, tt.sampleRate)
			if got != tt.want {
				t.Fatalf("shouldRunRemotePromptAudit() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSampledPromptAuditUsesStablePercentageBucket(t *testing.T) {
	const attempts = 10000
	matches := 0
	for i := 0; i < attempts; i++ {
		if sampledPromptAudit(42, fmt.Sprintf("request-%d", i), int64(i+1), 10) {
			matches++
		}
	}
	if matches < 900 || matches > 1100 {
		t.Fatalf("10 percent sample selected %d/%d requests", matches, attempts)
	}
	if sampledPromptAudit(42, "request", 1, 0) {
		t.Fatal("zero sample rate must never select")
	}
	if !sampledPromptAudit(42, "request", 1, 100) {
		t.Fatal("100 percent sample rate must always select")
	}
}
