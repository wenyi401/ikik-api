package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestParseAccountSharingPaginationAllowsOneThousandRows(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/?account_page=2&account_page_size=1000", nil)

	page, pageSize, err := parseAccountSharingPagination(c)
	require.NoError(t, err)
	require.Equal(t, 2, page)
	require.Equal(t, 1000, pageSize)
}

func TestParseAccountSharingPaginationRejectsMoreThanOneThousandRows(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/?account_page_size=1001", nil)

	_, _, err := parseAccountSharingPagination(c)
	require.ErrorContains(t, err, "1-1000")
}
