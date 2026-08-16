package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAPIKeyAuthSnapshotPreservesGroupPricingPolicy(t *testing.T) {
	groupID := int64(51)
	baseInput := 5e-6
	highInput := 10e-6
	threshold := 272000
	group := &Group{
		ID:                        groupID,
		Name:                      "gpt pricing",
		Platform:                  PlatformOpenAI,
		Status:                    StatusActive,
		Hydrated:                  true,
		SubscriptionType:          SubscriptionTypeStandard,
		RateMultiplier:            1,
		LongContextPricingEnabled: true,
		ModelPricing: []ChannelModelPricing{{
			Models:      []string{"gpt-5.6-sol"},
			BillingMode: BillingModeToken,
			InputPrice:  &baseInput,
			Intervals: []PricingInterval{{
				MinTokens:  threshold,
				InputPrice: &highInput,
			}},
		}},
	}
	apiKey := &APIKey{
		ID: 81, UserID: 41, GroupID: &groupID, Key: "sk-pricing-roundtrip",
		Name: "pricing-roundtrip", Status: StatusActive,
		User:  &User{ID: 41, Status: StatusActive, Concurrency: 5},
		Group: group,
		GroupRoutes: []APIKeyGroupRoute{{
			ID: 1, APIKeyID: 81, GroupID: groupID, Enabled: true, Group: group,
		}},
	}

	svc := &APIKeyService{}
	snapshot := svc.snapshotFromAPIKey(context.Background(), apiKey)
	require.Equal(t, apiKeyAuthSnapshotVersion, snapshot.Version)

	payload, err := json.Marshal(&APIKeyAuthCacheEntry{Snapshot: snapshot})
	require.NoError(t, err)
	var entry APIKeyAuthCacheEntry
	require.NoError(t, json.Unmarshal(payload, &entry))

	restored, used, err := svc.applyAuthCacheEntry(apiKey.Key, &entry)
	require.NoError(t, err)
	require.True(t, used)
	require.True(t, restored.Group.LongContextPricingEnabled)
	require.Len(t, restored.Group.ModelPricing, 1)
	require.InDelta(t, baseInput, *restored.Group.ModelPricing[0].InputPrice, 1e-12)
	require.Equal(t, threshold, restored.Group.ModelPricing[0].Intervals[0].MinTokens)
	require.InDelta(t, highInput, *restored.Group.ModelPricing[0].Intervals[0].InputPrice, 1e-12)

	require.Len(t, restored.GroupRoutes, 1)
	require.True(t, restored.GroupRoutes[0].Group.LongContextPricingEnabled)
	require.Len(t, restored.GroupRoutes[0].Group.ModelPricing, 1)
}
