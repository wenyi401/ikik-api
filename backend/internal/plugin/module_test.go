package plugin

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestModuleIDNamespaceAndName(t *testing.T) {
	tests := []struct {
		id        ModuleID
		namespace string
		name      string
	}{
		{id: "job.hello", namespace: "job", name: "hello"},
		{id: "gateway.platform.anthropic", namespace: "gateway.platform", name: "anthropic"},
		{id: "hello", namespace: "", name: "hello"},
		{id: "payment.provider.stripe-checkout", namespace: "payment.provider", name: "stripe-checkout"},
	}
	for _, tt := range tests {
		t.Run(string(tt.id), func(t *testing.T) {
			require.Equal(t, tt.namespace, tt.id.Namespace())
			require.Equal(t, tt.name, tt.id.Name())
		})
	}
}

func TestModuleIDValidate(t *testing.T) {
	valid := []ModuleID{
		"hello",
		"job.hello",
		"gateway.platform.anthropic",
		"a.b-c.d_e",
		"ns.mod123",
	}
	for _, id := range valid {
		require.NoError(t, id.validate(), "id %q should be valid", id)
	}

	invalid := []ModuleID{
		"",
		".",
		".hello",
		"hello.",
		"job..hello",
		"Job.hello",       // 大写
		"job.hello!",      // 非法字符
		"job. hello",      // 空格
		"job.héllo",       // 非 ASCII
		"job.hello.World", // 末段大写
	}
	for _, id := range invalid {
		require.Error(t, id.validate(), "id %q should be invalid", id)
	}
}
