package plugin

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseConfigEnabledTriState(t *testing.T) {
	cfg, err := ParseConfig(map[string]map[string]any{
		"job.on":      {"enabled": true},
		"job.off":     {"enabled": false},
		"job.unset":   {"greeting": "hi"},
		"job.strbool": {"enabled": "true"}, // 弱类型：字符串 bool（如来自环境变量）
	})
	require.NoError(t, err)

	require.NotNil(t, cfg["job.on"].Enabled)
	require.True(t, *cfg["job.on"].Enabled)

	require.NotNil(t, cfg["job.off"].Enabled)
	require.False(t, *cfg["job.off"].Enabled)

	require.Nil(t, cfg["job.unset"].Enabled, "未显式配置 enabled 时必须为 nil（用模块默认值）")

	require.NotNil(t, cfg["job.strbool"].Enabled)
	require.True(t, *cfg["job.strbool"].Enabled)
}

func TestParseConfigSeparatesEnabledFromRaw(t *testing.T) {
	cfg, err := ParseConfig(map[string]map[string]any{
		"job.hello": {"enabled": true, "greeting": "hi", "interval": "5s"},
	})
	require.NoError(t, err)

	mc := cfg["job.hello"]
	require.Equal(t, map[string]any{"greeting": "hi", "interval": "5s"}, mc.Raw,
		"enabled 键属于内核，不得混入模块私有配置")
}

func TestParseConfigErrors(t *testing.T) {
	t.Run("invalid module ID", func(t *testing.T) {
		_, err := ParseConfig(map[string]map[string]any{"Job.Hello": {}})
		require.Error(t, err)
		require.Contains(t, err.Error(), "Job.Hello")
	})
	t.Run("invalid enabled type", func(t *testing.T) {
		_, err := ParseConfig(map[string]map[string]any{"job.hello": {"enabled": 123}})
		require.Error(t, err)
		require.Contains(t, err.Error(), "job.hello")
	})
	t.Run("invalid enabled string", func(t *testing.T) {
		_, err := ParseConfig(map[string]map[string]any{"job.hello": {"enabled": "not-a-bool"}})
		require.Error(t, err)
	})
	t.Run("unregistered but valid ID is allowed", func(t *testing.T) {
		cfg, err := ParseConfig(map[string]map[string]any{"future.module": {"enabled": true}})
		require.NoError(t, err)
		require.Contains(t, cfg, ModuleID("future.module"))
	})
}

type helloModuleConfig struct {
	Greeting string        `mapstructure:"greeting"`
	Interval time.Duration `mapstructure:"interval"`
	Targets  []string      `mapstructure:"targets"`
	Limit    int           `mapstructure:"limit"`
}

func TestConfigOfDecodesRawIntoStruct(t *testing.T) {
	cfg, err := ParseConfig(map[string]map[string]any{
		"job.hello": {
			"enabled":  true,
			"greeting": "hi",
			"interval": "5s",    // 字符串 → time.Duration（与 viper 默认钩子一致）
			"targets":  "a,b,c", // 逗号分隔 → 切片（与 viper 默认钩子一致）
			"limit":    "42",    // 弱类型：字符串 → int
		},
	})
	require.NoError(t, err)

	var out helloModuleConfig
	require.NoError(t, cfg.Of("job.hello", &out))
	require.Equal(t, "hi", out.Greeting)
	require.Equal(t, 5*time.Second, out.Interval)
	require.Equal(t, []string{"a", "b", "c"}, out.Targets)
	require.Equal(t, 42, out.Limit)
}

func TestConfigOfMissingModuleLeavesOutUntouched(t *testing.T) {
	cfg := Config{}
	out := helloModuleConfig{Greeting: "default-value"}
	require.NoError(t, cfg.Of("job.absent", &out))
	require.Equal(t, "default-value", out.Greeting, "模块未配置时不得修改 out（保留模块默认值）")
}

func TestConfigOfTypeErrorContainsModuleID(t *testing.T) {
	cfg, err := ParseConfig(map[string]map[string]any{
		"job.hello": {"limit": "not-an-int"},
	})
	require.NoError(t, err)

	var out helloModuleConfig
	err = cfg.Of("job.hello", &out)
	require.Error(t, err)
	require.Contains(t, err.Error(), "job.hello", "解码错误必须包含模块 ID")
}

func TestConfigEnabledFor(t *testing.T) {
	on, off := true, false
	cfg := Config{
		"job.explicit-on":  {Enabled: &on},
		"job.explicit-off": {Enabled: &off},
		"job.mentioned":    {},
	}

	tests := []struct {
		name string
		info ModuleInfo
		want bool
	}{
		{"explicit on overrides default off", ModuleInfo{ID: "job.explicit-on", EnabledByDefault: false}, true},
		{"explicit off overrides default on", ModuleInfo{ID: "job.explicit-off", EnabledByDefault: true}, false},
		{"mentioned without enabled uses default true", ModuleInfo{ID: "job.mentioned", EnabledByDefault: true}, true},
		{"mentioned without enabled uses default false", ModuleInfo{ID: "job.mentioned", EnabledByDefault: false}, false},
		{"absent uses default true", ModuleInfo{ID: "job.absent", EnabledByDefault: true}, true},
		{"absent uses default false", ModuleInfo{ID: "job.absent", EnabledByDefault: false}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, cfg.enabledFor(tt.info))
		})
	}
}
