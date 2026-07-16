package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestWriteKiroModelsListUsesKiroModelShape(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	writeKiroModelsList(c, []string{"claude-sonnet-4-6", "custom-model"})

	require.Equal(t, http.StatusOK, rec.Code)
	var resp struct {
		Object string `json:"object"`
		Data   []struct {
			ID          string `json:"id"`
			Object      string `json:"object"`
			OwnedBy     string `json:"owned_by"`
			DisplayName string `json:"display_name"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, "list", resp.Object)
	require.Len(t, resp.Data, 2)
	require.Equal(t, "claude-sonnet-4-6", resp.Data[0].ID)
	require.Equal(t, "model", resp.Data[0].Object)
	require.Equal(t, "kiro", resp.Data[0].OwnedBy)
	require.Equal(t, "Claude Sonnet 4.6", resp.Data[0].DisplayName)
	require.Equal(t, "custom-model", resp.Data[1].ID)
	require.Equal(t, "kiro", resp.Data[1].OwnedBy)
	require.Equal(t, "custom-model", resp.Data[1].DisplayName)
}
