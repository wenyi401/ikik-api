package service

type APIKeyAuthGroupRouteSnapshot struct {
	ID              int64                    `json:"id"`
	APIKeyID        int64                    `json:"api_key_id"`
	GroupID         int64                    `json:"group_id"`
	Priority        int                      `json:"priority"`
	Weight          int                      `json:"weight"`
	Enabled         bool                     `json:"enabled"`
	CooldownSeconds int                      `json:"cooldown_seconds"`
	Group           *APIKeyAuthGroupSnapshot `json:"group,omitempty"`
}

func attachAPIKeyGroupRoutesToAuthSnapshot(snapshot *APIKeyAuthSnapshot, apiKey *APIKey) {
	if snapshot == nil || apiKey == nil || len(apiKey.GroupRoutes) == 0 {
		return
	}
	snapshot.GroupRoutes = make([]APIKeyAuthGroupRouteSnapshot, 0, len(apiKey.GroupRoutes))
	for i := range apiKey.GroupRoutes {
		route := &apiKey.GroupRoutes[i]
		snapshot.GroupRoutes = append(snapshot.GroupRoutes, APIKeyAuthGroupRouteSnapshot{
			ID:              route.ID,
			APIKeyID:        route.APIKeyID,
			GroupID:         route.GroupID,
			Priority:        route.Priority,
			Weight:          route.Weight,
			Enabled:         route.Enabled,
			CooldownSeconds: route.CooldownSeconds,
			Group:           apiKeyAuthGroupSnapshotFromGroup(route.Group),
		})
	}
}

func attachAPIKeyGroupRoutesFromAuthSnapshot(apiKey *APIKey, snapshot *APIKeyAuthSnapshot) {
	if apiKey == nil || snapshot == nil || len(snapshot.GroupRoutes) == 0 {
		return
	}
	apiKey.GroupRoutes = make([]APIKeyGroupRoute, 0, len(snapshot.GroupRoutes))
	for i := range snapshot.GroupRoutes {
		route := &snapshot.GroupRoutes[i]
		apiKey.GroupRoutes = append(apiKey.GroupRoutes, APIKeyGroupRoute{
			ID:              route.ID,
			APIKeyID:        route.APIKeyID,
			GroupID:         route.GroupID,
			Priority:        route.Priority,
			Weight:          route.Weight,
			Enabled:         route.Enabled,
			CooldownSeconds: route.CooldownSeconds,
			Group:           groupFromAPIKeyAuthSnapshot(route.Group),
		})
	}
}

func apiKeyAuthGroupSnapshotFromGroup(group *Group) *APIKeyAuthGroupSnapshot {
	if group == nil {
		return nil
	}
	return &APIKeyAuthGroupSnapshot{
		ID:                              group.ID,
		Name:                            group.Name,
		Platform:                        group.Platform,
		IsExclusive:                     group.IsExclusive,
		Status:                          group.Status,
		SubscriptionType:                group.SubscriptionType,
		RateMultiplier:                  group.RateMultiplier,
		DailyLimitUSD:                   group.DailyLimitUSD,
		WeeklyLimitUSD:                  group.WeeklyLimitUSD,
		MonthlyLimitUSD:                 group.MonthlyLimitUSD,
		AllowImageGeneration:            group.AllowImageGeneration,
		AllowBatchImageGeneration:       group.AllowBatchImageGeneration,
		ImageRateIndependent:            group.ImageRateIndependent,
		ImageRateMultiplier:             group.ImageRateMultiplier,
		ImagePrice1K:                    group.ImagePrice1K,
		ImagePrice2K:                    group.ImagePrice2K,
		ImagePrice4K:                    group.ImagePrice4K,
		VideoRateIndependent:            group.VideoRateIndependent,
		VideoRateMultiplier:             group.VideoRateMultiplier,
		VideoPrice480P:                  group.VideoPrice480P,
		VideoPrice720P:                  group.VideoPrice720P,
		VideoPrice1080P:                 group.VideoPrice1080P,
		WebSearchPricePerCall:           group.WebSearchPricePerCall,
		ClaudeCodeOnly:                  group.ClaudeCodeOnly,
		FallbackGroupID:                 group.FallbackGroupID,
		FallbackGroupIDOnInvalidRequest: group.FallbackGroupIDOnInvalidRequest,
		ModelRouting:                    group.ModelRouting,
		ModelRoutingEnabled:             group.ModelRoutingEnabled,
		MCPXMLInject:                    group.MCPXMLInject,
		SupportedModelScopes:            group.SupportedModelScopes,
		AllowMessagesDispatch:           group.AllowMessagesDispatch,
		DefaultMappedModel:              group.DefaultMappedModel,
		MessagesDispatchModelConfig:     group.MessagesDispatchModelConfig,
		ModelsListConfig:                group.ModelsListConfig,
		OpenAIExperimentalPromptEnabled: group.OpenAIExperimentalPromptEnabled,
		RPMLimit:                        group.RPMLimit,
		PeakRateEnabled:                 group.PeakRateEnabled,
		PeakStart:                       group.PeakStart,
		PeakEnd:                         group.PeakEnd,
		PeakRateMultiplier:              group.PeakRateMultiplier,
	}
}

func groupFromAPIKeyAuthSnapshot(snapshot *APIKeyAuthGroupSnapshot) *Group {
	if snapshot == nil {
		return nil
	}
	return &Group{
		ID:                              snapshot.ID,
		Name:                            snapshot.Name,
		Platform:                        snapshot.Platform,
		IsExclusive:                     snapshot.IsExclusive,
		Status:                          snapshot.Status,
		Hydrated:                        true,
		SubscriptionType:                snapshot.SubscriptionType,
		RateMultiplier:                  snapshot.RateMultiplier,
		DailyLimitUSD:                   snapshot.DailyLimitUSD,
		WeeklyLimitUSD:                  snapshot.WeeklyLimitUSD,
		MonthlyLimitUSD:                 snapshot.MonthlyLimitUSD,
		AllowImageGeneration:            snapshot.AllowImageGeneration,
		AllowBatchImageGeneration:       snapshot.AllowBatchImageGeneration,
		ImageRateIndependent:            snapshot.ImageRateIndependent,
		ImageRateMultiplier:             snapshot.ImageRateMultiplier,
		ImagePrice1K:                    snapshot.ImagePrice1K,
		ImagePrice2K:                    snapshot.ImagePrice2K,
		ImagePrice4K:                    snapshot.ImagePrice4K,
		VideoRateIndependent:            snapshot.VideoRateIndependent,
		VideoRateMultiplier:             snapshot.VideoRateMultiplier,
		VideoPrice480P:                  snapshot.VideoPrice480P,
		VideoPrice720P:                  snapshot.VideoPrice720P,
		VideoPrice1080P:                 snapshot.VideoPrice1080P,
		WebSearchPricePerCall:           snapshot.WebSearchPricePerCall,
		ClaudeCodeOnly:                  snapshot.ClaudeCodeOnly,
		FallbackGroupID:                 snapshot.FallbackGroupID,
		FallbackGroupIDOnInvalidRequest: snapshot.FallbackGroupIDOnInvalidRequest,
		ModelRouting:                    snapshot.ModelRouting,
		ModelRoutingEnabled:             snapshot.ModelRoutingEnabled,
		MCPXMLInject:                    snapshot.MCPXMLInject,
		SupportedModelScopes:            snapshot.SupportedModelScopes,
		AllowMessagesDispatch:           snapshot.AllowMessagesDispatch,
		DefaultMappedModel:              snapshot.DefaultMappedModel,
		MessagesDispatchModelConfig:     snapshot.MessagesDispatchModelConfig,
		ModelsListConfig:                snapshot.ModelsListConfig,
		OpenAIExperimentalPromptEnabled: snapshot.OpenAIExperimentalPromptEnabled,
		RPMLimit:                        snapshot.RPMLimit,
		PeakRateEnabled:                 snapshot.PeakRateEnabled,
		PeakStart:                       snapshot.PeakStart,
		PeakEnd:                         snapshot.PeakEnd,
		PeakRateMultiplier:              snapshot.PeakRateMultiplier,
	}
}
