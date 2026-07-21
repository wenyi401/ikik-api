package handler

import (
	"ikik-api/internal/service"
)

func channelMonitorCapacitySummary(items []service.GroupCapacitySummary) channelMonitorCapacitySummaryResponse {
	total := service.GroupCapacitySummary{}
	for _, item := range items {
		total.ConcurrencyUsed += item.ConcurrencyUsed
		total.ConcurrencyMax += item.ConcurrencyMax
		total.SessionsUsed += item.SessionsUsed
		total.SessionsMax += item.SessionsMax
		total.RPMUsed += item.RPMUsed
		total.RPMMax += item.RPMMax
	}
	return channelMonitorCapacitySummaryResponse{
		Items: items,
		Total: total,
	}
}

type channelMonitorCapacitySummaryResponse struct {
	Items []service.GroupCapacitySummary `json:"items"`
	Total service.GroupCapacitySummary   `json:"total"`
}
