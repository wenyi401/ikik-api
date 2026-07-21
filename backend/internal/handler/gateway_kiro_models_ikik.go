package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"ikik-api/internal/pkg/kiro"
)

func defaultKiroModelIDs() []string {
	return kiro.DefaultModelIDs()
}

func writeKiroModelsList(c *gin.Context, modelIDs []string) {
	defaults := kiro.DefaultModels()
	defaultsByID := make(map[string]kiro.Model, len(defaults))
	for _, model := range defaults {
		defaultsByID[model.ID] = model
	}

	models := make([]kiro.Model, 0, len(modelIDs))
	for _, modelID := range modelIDs {
		if model, ok := defaultsByID[modelID]; ok {
			models = append(models, model)
			continue
		}
		models = append(models, kiro.Model{
			ID:          modelID,
			Object:      "model",
			Created:     1704067200,
			OwnedBy:     "kiro",
			DisplayName: modelID,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   models,
	})
}
