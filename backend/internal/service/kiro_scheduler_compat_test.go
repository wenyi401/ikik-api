package service

import (
	"context"
	"testing"
)

func TestKiroMetadataAccountIsChatCompletionsSchedulable(t *testing.T) {
	account := &Account{
		ID:          12116,
		Name:        "kiro metadata account",
		Platform:    PlatformKiro,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 10,
		Priority:    1,
		GroupIDs:    []int64{15507},
	}

	svc := &OpenAIGatewayService{}
	if !svc.isOpenAIAccountEligibleForSchedulingRequest(
		context.Background(),
		account,
		PlatformKiro,
		"claude-sonnet-4-5-20250929",
		false,
		OpenAIEndpointCapabilityChatCompletions,
	) {
		t.Fatal("expected Kiro metadata account to be schedulable for chat completions")
	}
}
