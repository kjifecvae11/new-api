package router

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRelayRouterRegistersResponsesHTTPAndWebsocket(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetRelayRouter(engine)

	routes := engine.Routes()
	foundGet := false
	foundPost := false
	for _, route := range routes {
		if route.Path != "/v1/responses" {
			continue
		}
		switch route.Method {
		case "GET":
			foundGet = true
		case "POST":
			foundPost = true
		}
	}
	require.True(t, foundGet, "GET /v1/responses must provide WebSocket transport")
	require.True(t, foundPost, "POST /v1/responses must preserve HTTP transport")
}
