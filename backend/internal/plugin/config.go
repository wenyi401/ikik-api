package plugin

import (
	"fmt"
	"strconv"

	"github.com/mitchellh/mapstructure"
)

// enabledKey 是每个模块配置项中由内核保留解释的键，其余键全部属于模块私有配置。
const enabledKey = "enabled"

// ModuleConfig 是单个模块在 `modules:` 配置子树中的配置项。
type ModuleConfig struct {
	// Enabled 是模块的显式启停开关；nil 表示未显式配置，
	// 由模块注册时声明的 ModuleInfo.EnabledByDefault 决定。
	Enabled *bool

	// Raw 是模块的私有配置（去掉 enabled 键之后的剩余部分），
	// 由 Config.Of 解码到模块自定义的配置 struct，内核不解释其内容。
	Raw map[string]any
}

// Config 是 `modules:` 配置子树的解析结果：模块 ID → 模块配置。
//
// 缺省（子树不存在）时为空 map，所有模块按各自默认 enabled 值运行，
// 行为与未引入插件内核前完全一致。
type Config map[ModuleID]ModuleConfig

// ParseConfig 将全局配置中 `modules:` 子树的原始映射解析为 Config。
//
// raw 的顶层 key 必须是合法的模块 ID 字面量（如 "job.hello"，YAML 中带点的
// 完整 key），value 为该模块的配置映射；其中 enabled 键被内核解释为启停开关
// （接受 bool 或可解析为 bool 的字符串），其余键原样保留为模块私有配置。
//
// 允许出现"格式合法但未注册"的模块 ID（例如不同编译变体的配置共用），
// 这类配置项会被 Runtime 忽略；但 ID 格式非法或 enabled 类型错误会返回错误，
// 以便尽早暴露配置笔误。
//
// 注意：全局配置经 Viper 加载时，所有 key（含模块 ID 与私有配置键）会被
// 静默小写化——配置文件里写 "Job.Hello" 实际到达这里时已是 "job.hello"。
// 因此模块 ID 与私有配置 struct 的 mapstructure 标签必须全小写。
func ParseConfig(raw map[string]map[string]any) (Config, error) {
	cfg := make(Config, len(raw))
	for key, entry := range raw {
		id := ModuleID(key)
		if err := id.validate(); err != nil {
			return nil, fmt.Errorf("plugin config: invalid module ID in modules config: %w", err)
		}
		mc := ModuleConfig{Raw: make(map[string]any, len(entry))}
		for k, v := range entry {
			if k == enabledKey {
				// YAML 中写空的 `enabled:`（值为 null）按"未显式配置"处理，
				// 走模块默认值——与 Enabled=nil 的三态语义一致（审计 C-1）。
				if v == nil {
					continue
				}
				b, err := toBool(v)
				if err != nil {
					return nil, fmt.Errorf("plugin config: module %q: invalid enabled value: %w", id, err)
				}
				mc.Enabled = &b
				continue
			}
			mc.Raw[k] = v
		}
		cfg[id] = mc
	}
	return cfg, nil
}

// Of 将模块 id 的 raw 私有配置解码到 out（必须是指向配置 struct 的指针）。
//
// 解码行为与项目全局配置（Viper 默认解码器）保持一致：
// mapstructure 标签、弱类型转换、字符串到 time.Duration / 逗号分隔切片的钩子。
// 模块未配置或私有配置为空时不修改 out（模块应自带默认值并返回 nil）；
// 类型不匹配等解码失败时返回包含模块 ID 的错误。
func (c Config) Of(id ModuleID, out any) error {
	mc, ok := c[id]
	if !ok || len(mc.Raw) == 0 {
		return nil
	}
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:           out,
		WeaklyTypedInput: true,
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeDurationHookFunc(),
			mapstructure.StringToSliceHookFunc(","),
		),
	})
	if err != nil {
		return fmt.Errorf("plugin config: build decoder for module %q: %w", id, err)
	}
	if err := decoder.Decode(mc.Raw); err != nil {
		return fmt.Errorf("plugin config: decode config for module %q: %w", id, err)
	}
	return nil
}

// enabledFor 解析模块的最终启停状态：显式配置优先，否则用注册时声明的默认值。
func (c Config) enabledFor(info ModuleInfo) bool {
	if mc, ok := c[info.ID]; ok && mc.Enabled != nil {
		return *mc.Enabled
	}
	return info.EnabledByDefault
}

// toBool 将 enabled 配置值弱类型转换为 bool（接受 bool 与 strconv.ParseBool 可解析的字符串）。
func toBool(v any) (bool, error) {
	switch val := v.(type) {
	case bool:
		return val, nil
	case string:
		b, err := strconv.ParseBool(val)
		if err != nil {
			return false, fmt.Errorf("cannot parse %q as bool", val)
		}
		return b, nil
	default:
		return false, fmt.Errorf("expected bool, got %T", v)
	}
}
