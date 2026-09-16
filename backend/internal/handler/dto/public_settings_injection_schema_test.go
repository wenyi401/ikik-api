package dto

import (
	"reflect"
	"strings"
	"testing"

	"ikik-api/internal/service"
)

// TestPublicSettingsInjectionPayload_SchemaDoesNotDrift guarantees the SSR
// injection struct exposes every JSON field consumed by the frontend.
//
// Why this test exists: before we extracted a named PublicSettingsInjectionPayload
// type, the inline struct was manually kept in sync with dto.PublicSettings and
// drifted — ChannelMonitorEnabled / AvailableChannelsEnabled were missing, which
// made the frontend read `undefined` on refresh and hide the "可用渠道" menu
// until the async /api/v1/settings/public round-trip finished.
//
// This test compares the two JSON-tag sets and fails if injection is missing
// any field that dto.PublicSettings exposes. Adding a new feature flag with
// only a DTO entry will fail this test until the injection struct is updated.
//
// Intentional exclusions (fields present on dto.PublicSettings that SSR does
// not need to inject) are listed in `dtoOnlyFields` below with a reason.
func TestPublicSettingsInjectionPayload_SchemaDoesNotDrift(t *testing.T) {
	injection := jsonTags(reflect.TypeOf(service.PublicSettingsInjectionPayload{}))
	dtoKeys := jsonTags(reflect.TypeOf(PublicSettings{}))

	// Fields that legitimately live only on the DTO. Keep tiny; document each.
	dtoOnlyFields := map[string]string{
		// force_email_on_third_party_signup lives on the DTO but is not injected via SSR.
		"force_email_on_third_party_signup": "auth-source default, not a feature flag",
		// sora_client_enabled 是 fork 早期预留的公开开关：只有 DTO 声明，
		// 服务层没有对应配置项与写入方（恒为 false），前端也未消费，
		// 因此 SSR 注入不需要它。将来接入真实配置时，应改为写入
		// service.PublicSettingsInjectionPayload。
		"sora_client_enabled": "reserved flag without producer or consumer; nothing to inject",
	}

	var missing []string
	for key := range dtoKeys {
		if _, ok := injection[key]; ok {
			continue
		}
		if _, allowed := dtoOnlyFields[key]; allowed {
			continue
		}
		missing = append(missing, key)
	}
	if len(missing) > 0 {
		t.Fatalf("service.PublicSettingsInjectionPayload is missing JSON fields present on dto.PublicSettings: %s\n"+
			"add the field to PublicSettingsInjectionPayload (and GetPublicSettingsForInjection), or "+
			"document the exclusion in dtoOnlyFields with a reason.", strings.Join(missing, ", "))
	}
}

func jsonTags(t reflect.Type) map[string]struct{} {
	out := make(map[string]struct{})
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag := f.Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		name := strings.SplitN(tag, ",", 2)[0]
		if name == "" {
			continue
		}
		out[name] = struct{}{}
	}
	return out
}
