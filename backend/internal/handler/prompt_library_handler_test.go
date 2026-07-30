package handler

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizePromptLibraryQuery(t *testing.T) {
	query, cacheKey, err := normalizePromptLibraryQuery(url.Values{
		"page":     {"2"},
		"per_page": {"24"},
		"q":        {" coding assistant "},
		"sort":     {"upvotes"},
		"type":     {"image"},
	})
	require.NoError(t, err)
	require.Equal(t, "2", query.Get("page"))
	require.Equal(t, "24", query.Get("perPage"))
	require.Equal(t, "coding assistant", query.Get("q"))
	require.Equal(t, "upvotes", query.Get("sort"))
	require.Equal(t, "IMAGE", query.Get("type"))
	require.Equal(t, query.Encode(), cacheKey)
}

func TestNormalizePromptLibraryQueryRejectsUnsafeBounds(t *testing.T) {
	_, _, err := normalizePromptLibraryQuery(url.Values{"per_page": {"1000"}})
	require.ErrorContains(t, err, "per_page")

	_, _, err = normalizePromptLibraryQuery(url.Values{"sort": {"random"}})
	require.ErrorContains(t, err, "sort")

	_, _, err = normalizePromptLibraryQuery(url.Values{"type": {"unknown"}})
	require.ErrorContains(t, err, "type")
}
