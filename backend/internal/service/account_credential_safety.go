package service

import "strings"

type credentialSafetyOptions struct {
	AllowClaudeSessionKeyFields bool
	AllowOAuthTokenValues       bool
	AllowOAuthMetadataURLs      bool
}

func findDisallowedCredentialContent(value any, opts credentialSafetyOptions) (string, bool) {
	return findDisallowedCredentialContentAt(value, "", opts)
}

func findDisallowedCredentialContentAt(value any, parentKey string, opts credentialSafetyOptions) (string, bool) {
	switch typed := value.(type) {
	case map[string]any:
		for key, nested := range typed {
			normalizedKey := normalizeCredentialSafetyKey(key)
			if isDisallowedCredentialSafetyFieldKey(normalizedKey, opts) {
				return key, true
			}
			if text, ok := nested.(string); ok {
				if _, blocked := disallowedCredentialStringReason(normalizedKey, text, opts); blocked {
					return key, true
				}
			}
			if field, ok := findDisallowedCredentialContentAt(nested, normalizedKey, opts); ok {
				return field, true
			}
		}
	case []any:
		for _, item := range typed {
			if field, ok := findDisallowedCredentialContentAt(item, parentKey, opts); ok {
				return field, true
			}
		}
	case string:
		if _, blocked := disallowedCredentialStringReason(parentKey, typed, opts); blocked {
			if parentKey != "" {
				return parentKey, true
			}
			return "credential", true
		}
	}
	return "", false
}

func isDisallowedCredentialSafetyFieldKey(normalizedKey string, opts credentialSafetyOptions) bool {
	switch normalizedKey {
	case "api_key",
		"apikey",
		"x_api_key",
		"xapikey",
		"authorization",
		"authorization_header",
		"authorizationheader",
		"base_url",
		"baseurl",
		"api_base_url",
		"api_baseurl",
		"custom_base_url",
		"custom_baseurl",
		"custom_base_url_enabled",
		"custom_baseurl_enabled",
		"upstream",
		"upstream_url",
		"upstreamurl",
		"upstream_base_url",
		"upstream_baseurl",
		"upstream_endpoint",
		"upstreamendpoint",
		"endpoint",
		"endpoint_url",
		"endpointurl",
		"url",
		"host",
		"proxy_url",
		"proxyurl",
		"cookie",
		"cookies",
		"set_cookie",
		"setcookie",
		"auth_mode",
		"authmode",
		"aws_access_key_id",
		"awsaccesskeyid",
		"aws_secret_access_key",
		"awssecretaccesskey",
		"aws_session_token",
		"awssessiontoken",
		"access_key_id",
		"accesskeyid",
		"secret_access_key":
		return true
	case "session_key", "sessionkey", "session_token", "claude_session_key":
		return !opts.AllowClaudeSessionKeyFields
	}
	return false
}

func disallowedCredentialStringReason(key, value string, opts credentialSafetyOptions) (string, bool) {
	text := strings.TrimSpace(value)
	if text == "" {
		return "", false
	}
	lower := strings.ToLower(text)
	if opts.AllowClaudeSessionKeyFields && isClaudeSessionKeyField(key) {
		if _, ok := extractClaudeSessionKey(text); ok {
			return "", false
		}
	}
	if opts.AllowOAuthMetadataURLs && isAllowedOAuthMetadataURLField(key) {
		return "", false
	}
	switch {
	case strings.Contains(lower, "http://") || strings.Contains(lower, "https://"):
		return "URL is not allowed", true
	case containsForbiddenCredentialText(lower):
		return "forbidden credential field text is not allowed", true
	case isAPIKeyLikeCredentialValue(text, key, opts):
		return "API key-like credential is not allowed", true
	}
	return "", false
}

func isAllowedOAuthMetadataURLField(key string) bool {
	switch normalizeCredentialSafetyKey(key) {
	case "scope",
		"issuer",
		"iss",
		"picture",
		"avatar",
		"avatar_url",
		"avatarurl",
		"profile",
		"profile_url",
		"profileurl":
		return true
	default:
		return false
	}
}

func containsForbiddenCredentialText(lower string) bool {
	for _, needle := range []string{
		"authorization:",
		"authorization=",
		"bearer ",
		"api_key",
		"apikey",
		"x-api-key",
		"x_api_key",
		"base_url",
		"baseurl",
		"api_base_url",
		"api_baseurl",
		"custom_base_url",
		"custom_baseurl",
		"upstream_url",
		"upstreamurl",
		"upstream_base_url",
		"upstream_baseurl",
		"upstream_endpoint",
		"upstreamendpoint",
		"proxy_url",
		"proxyurl",
		"cookie:",
		"cookie=",
		"cookies:",
		"cookies=",
		"set-cookie",
		"auth_mode",
		"authmode",
		"aws_access_key_id",
		"awsaccesskeyid",
		"aws_secret_access_key",
		"awssecretaccesskey",
		"aws_session_token",
		"awssessiontoken",
		"access_key_id",
		"accesskeyid",
		"secret_access_key",
		"secretaccesskey",
	} {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	for _, prefix := range []string{"endpoint", "host", "url"} {
		if strings.Contains(lower, prefix+"=") || strings.Contains(lower, prefix+":") {
			return true
		}
	}
	return false
}

func isAPIKeyLikeCredentialValue(value, key string, opts credentialSafetyOptions) bool {
	text := strings.TrimSpace(strings.Trim(value, `"'`))
	if text == "" {
		return false
	}
	lower := strings.ToLower(text)
	switch {
	case strings.HasPrefix(lower, "sk-proj-"),
		strings.HasPrefix(lower, "sk-openai-"),
		strings.HasPrefix(lower, "sk-ant-api"),
		strings.HasPrefix(text, "AIza"),
		strings.HasPrefix(text, "AKIA"),
		strings.HasPrefix(text, "ASIA"):
		return true
	case strings.HasPrefix(lower, "sk-"):
		return !isAllowedStructuredOAuthSKToken(key, lower, opts)
	}
	return false
}

func isAllowedStructuredOAuthSKToken(key, lowerValue string, opts credentialSafetyOptions) bool {
	if opts.AllowOAuthTokenValues && key == "access_token" && strings.HasPrefix(lowerValue, "sk-ant-oat") {
		return true
	}
	if opts.AllowClaudeSessionKeyFields && isClaudeSessionKeyField(key) && strings.HasPrefix(lowerValue, "sk-ant-sid") {
		return true
	}
	return false
}

func isClaudeSessionKeyField(key string) bool {
	switch normalizeCredentialSafetyKey(key) {
	case "session_key", "sessionkey", "session_token", "claude_session_key":
		return true
	default:
		return false
	}
}

func normalizeCredentialSafetyKey(key string) string {
	return strings.NewReplacer("-", "_", ".", "_").Replace(strings.ToLower(strings.TrimSpace(key)))
}
