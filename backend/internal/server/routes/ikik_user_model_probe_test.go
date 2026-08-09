package routes

import (
	"testing"

	"ikik-api/internal/handler"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestIkikUserAccountRoutesRegisterModelProbeEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	h := &handler.Handlers{
		UserAccount: handler.NewUserAccountHandler(nil, nil, nil, nil, nil, nil, nil),
	}
	registerIkikUserAccountRoutes(router.Group("/api/v1"), h)

	routes := make(map[string]struct{})
	for _, route := range router.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}
	require.Contains(t, routes, "POST /api/v1/accounts/model-probe/list")
	require.Contains(t, routes, "POST /api/v1/accounts/model-probe/test")
}
