package common

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestGetRequestBodyRejectsMissingRequestBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = &http.Request{}

	if _, err := GetRequestBody(ctx); err == nil {
		t.Fatal("GetRequestBody() error = nil, want missing-body error")
	}
}
