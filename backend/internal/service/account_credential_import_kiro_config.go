package service

import (
	"fmt"
	"strings"
)

const kiroConfigImportSource = "kiro_config"

func accountCredentialImportSourceFromKiroConfig(item map[string]any) (AccountCredentialImportSource, bool, error) {
	clientID := importStringField(item, "client_id", "clientId")
	clientSecret := importStringField(item, "client_secret", "clientSecret")
	refreshToken := importStringField(item, "refresh_token", "refreshToken")
	if clientID == "" && clientSecret == "" && refreshToken == "" {
		return AccountCredentialImportSource{}, false, nil
	}
	if clientID == "" || clientSecret == "" || refreshToken == "" {
		return AccountCredentialImportSource{}, true, errKiroConfigImportIncomplete()
	}

	email := importStringField(item, "email", "email_address", "emailAddress")
	provider := importStringField(item, "provider")
	if provider == "" {
		provider = "BuilderId"
	}
	region := importStringField(item, "region")
	if region == "" {
		region = "us-east-1"
	}
	startURL := importStringField(item, "start_url", "startURL")
	subscription := importStringField(item, "subscription", "plan_type", "planType")

	credentials := map[string]any{
		"auth_method":   "idc",
		"client_id":     clientID,
		"client_secret": clientSecret,
		"refresh_token": refreshToken,
		"provider":      provider,
		"region":        region,
		"import_source": kiroConfigImportSource,
	}
	if email != "" {
		credentials["email"] = email
	}
	if startURL != "" {
		credentials["start_url"] = startURL
	}
	if subscription != "" {
		credentials["plan_type"] = subscription
		credentials["subscription"] = subscription
	}
	copyCredentialNumberIfPresent(credentials, item, "credit_limit", "creditLimit")
	copyCredentialNumberIfPresent(credentials, item, "credit_used", "creditUsed")

	extra := map[string]any{
		"import_source":              kiroConfigImportSource,
		"openai_responses_supported": false,
	}
	name := strings.TrimSpace(importStringField(item, "name", "label"))
	if name == "" {
		name = email
	}

	return AccountCredentialImportSource{
		Kind:         AccountCredentialImportKindKiroConfig,
		Name:         name,
		Platform:     PlatformKiro,
		Credentials:  credentials,
		Extra:        extra,
		Token:        refreshToken,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		AuthMethod:   "idc",
		Provider:     provider,
		StartURL:     startURL,
		Region:       region,
		ProfileArn:   importStringField(item, "profile_arn", "profileArn"),
	}, true, nil
}

func errKiroConfigImportIncomplete() error {
	return fmt.Errorf("kiro config import requires clientId, clientSecret and refreshToken")
}

func copyCredentialNumberIfPresent(dst map[string]any, src map[string]any, dstKey string, sourceKeys ...string) {
	value, ok := importAnyField(src, sourceKeys...)
	if !ok {
		return
	}
	dst[dstKey] = value
}
