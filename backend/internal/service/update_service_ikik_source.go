package service

import (
	"fmt"
	"net/url"
	"strings"
)

func validateUpdateReleaseSource(release *GitHubRelease) error {
	if release == nil {
		return fmt.Errorf("release metadata is empty")
	}
	if err := validateUpdateRepositoryURL(release.HTMLURL); err != nil {
		return fmt.Errorf("release source mismatch: %w", err)
	}
	for _, asset := range release.Assets {
		if err := validateUpdateRepositoryURL(asset.BrowserDownloadURL); err != nil {
			return fmt.Errorf("release asset %q source mismatch: %w", asset.Name, err)
		}
	}
	return nil
}

func validateUpdateRepositoryURL(rawURL string) error {
	parsedURL, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if parsedURL.Scheme != "https" || !strings.EqualFold(parsedURL.Hostname(), allowedDownloadHost) {
		return fmt.Errorf("URL must use https://%s", allowedDownloadHost)
	}

	expectedPrefix := "/" + strings.ToLower(githubRepo) + "/releases/"
	if !strings.HasPrefix(strings.ToLower(parsedURL.EscapedPath()), expectedPrefix) {
		return fmt.Errorf("expected repository %s", githubRepo)
	}
	return nil
}
