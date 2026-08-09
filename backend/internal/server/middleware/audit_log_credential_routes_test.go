package middleware

import "testing"

func TestCredentialImportRoutesOmitAuditBodies(t *testing.T) {
	routes := []string{
		"POST /api/v1/admin/accounts/import/codex-session",
		"POST /api/v1/accounts/data",
		"POST /api/v1/accounts/import-agent-identity",
		"POST /api/v1/accounts/import-credentials",
		"POST /api/v1/accounts/model-probe/list",
		"POST /api/v1/accounts/model-probe/test",
		"POST /api/developer/v1/account-imports",
	}

	for _, route := range routes {
		if _, ok := auditBodyOmittedRoutes[route]; !ok {
			t.Errorf("credential-bearing route %q does not omit its audit body", route)
		}
	}
}
