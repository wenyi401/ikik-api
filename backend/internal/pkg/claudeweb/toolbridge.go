package claudeweb

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const (
	ToolChoiceAuto     = "auto"
	ToolChoiceNone     = "none"
	ToolChoiceRequired = "required"
	ToolChoiceSpecific = "specific"

	toolBridgeThinkingOpen  = "<think>"
	toolBridgeThinkingClose = "</think>"
	toolBridgeCallsOpen     = "<tool_calls>"
	toolBridgeCallsClose    = "</tool_calls>"
	toolBridgeCallOpen      = "<tool_call>"
	toolBridgeCallClose     = "</tool_call>"
	toolBridgeFinalOpen     = "<final_answer>"
	toolBridgeFinalClose    = "</final_answer>"

	// Keep accepting the original protocol for conversations that were started
	// before the Web2API-compatible bridge was introduced.
	legacyToolBridgeCallsOpen  = "<ikik_tool_calls>"
	legacyToolBridgeCallsClose = "</ikik_tool_calls>"
	legacyToolBridgeCallOpen   = "<ikik_tool_call>"
	legacyToolBridgeCallClose  = "</ikik_tool_call>"
	legacyToolBridgeFinalOpen  = "<ikik_final>"
	legacyToolBridgeFinalClose = "</ikik_final>"
	maxToolBridgeTools         = 128
	maxToolBridgePrompt        = 512 * 1024
	maxToolBridgeArguments     = 1024 * 1024
)

// ToolDefinition is the protocol-neutral function definition exposed to Claude Web.
type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// ToolBridgeConfig controls prompt-based tool calling for the private Claude Web API.
// Claude Web does not expose a documented client-tool protocol, so the bridge keeps
// the compatibility behavior isolated from the normal Anthropic gateway path.
type ToolBridgeConfig struct {
	Tools         []ToolDefinition
	ChoiceMode    string
	ForcedTool    string
	AllowParallel bool
	toolNames     map[string]struct{}
}

type toolChoicePayload struct {
	Type                   string `json:"type"`
	Name                   string `json:"name"`
	DisableParallelToolUse bool   `json:"disable_parallel_tool_use"`
}

// ToolCall is a validated tool invocation produced by the tagged bridge protocol.
type ToolCall struct {
	Name      string
	Arguments json.RawMessage
}

type taggedToolCall struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type ToolBridgeRequestError struct {
	err error
}

func (e *ToolBridgeRequestError) Error() string {
	if e == nil || e.err == nil {
		return "invalid claude Web tool request"
	}
	return e.err.Error()
}

func (e *ToolBridgeRequestError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func NewToolBridgeConfig(tools []ToolDefinition, rawChoice json.RawMessage) (config *ToolBridgeConfig, err error) {
	defer func() {
		if err != nil {
			err = &ToolBridgeRequestError{err: err}
		}
	}()
	if len(tools) == 0 {
		return nil, nil
	}
	if len(tools) > maxToolBridgeTools {
		return nil, fmt.Errorf("claude Web supports at most %d client tools", maxToolBridgeTools)
	}

	config = &ToolBridgeConfig{
		ChoiceMode:    ToolChoiceAuto,
		AllowParallel: true,
		toolNames:     make(map[string]struct{}, len(tools)),
	}
	for _, tool := range tools {
		tool.Name = strings.TrimSpace(tool.Name)
		tool.Description = strings.TrimSpace(tool.Description)
		if tool.Name == "" {
			return nil, errors.New("claude Web tool name is required")
		}
		if _, exists := config.toolNames[tool.Name]; exists {
			return nil, fmt.Errorf("duplicate claude Web tool %q", tool.Name)
		}
		if len(tool.Name) > 128 {
			return nil, fmt.Errorf("claude Web tool name %q is too long", tool.Name)
		}
		if descriptionRunes := []rune(tool.Description); len(descriptionRunes) > 4096 {
			tool.Description = string(descriptionRunes[:4096])
		}
		tool.InputSchema = normalizeToolBridgeSchema(tool.InputSchema)
		config.toolNames[tool.Name] = struct{}{}
		config.Tools = append(config.Tools, tool)
	}

	if len(bytes.TrimSpace(rawChoice)) > 0 && string(bytes.TrimSpace(rawChoice)) != "null" {
		if err := config.applyToolChoice(rawChoice); err != nil {
			return nil, err
		}
	}
	if config.ChoiceMode == ToolChoiceSpecific {
		if _, ok := config.toolNames[config.ForcedTool]; !ok {
			return nil, fmt.Errorf("claude Web tool_choice references unknown tool %q", config.ForcedTool)
		}
		// Claude Web only receives the selected tool for a specific choice. This
		// keeps the model-facing prompt simple while the parser still validates the
		// requested tool name below.
		config.Tools = filterToolBridgeTools(config.Tools, config.ForcedTool)
		config.toolNames = map[string]struct{}{config.ForcedTool: {}}
		config.AllowParallel = false
	}
	return config, nil
}

func filterToolBridgeTools(tools []ToolDefinition, name string) []ToolDefinition {
	for _, tool := range tools {
		if tool.Name == name {
			return []ToolDefinition{tool}
		}
	}
	return nil
}

func normalizeToolBridgeSchema(raw json.RawMessage) json.RawMessage {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || !json.Valid(trimmed) {
		return json.RawMessage(`{"type":"object","properties":{}}`)
	}
	var schema map[string]any
	if json.Unmarshal(trimmed, &schema) != nil {
		return json.RawMessage(`{"type":"object","properties":{}}`)
	}
	if _, ok := schema["type"]; !ok {
		schema["type"] = "object"
	}
	if _, ok := schema["properties"]; !ok {
		schema["properties"] = map[string]any{}
	}
	normalized, err := json.Marshal(schema)
	if err != nil {
		return json.RawMessage(`{"type":"object","properties":{}}`)
	}
	return normalized
}

func (c *ToolBridgeConfig) applyToolChoice(raw json.RawMessage) error {
	var simple string
	if json.Unmarshal(raw, &simple) == nil {
		return c.setToolChoice(simple, "", false)
	}
	var choice toolChoicePayload
	if err := json.Unmarshal(raw, &choice); err != nil {
		return fmt.Errorf("decode claude Web tool_choice: %w", err)
	}
	return c.setToolChoice(choice.Type, choice.Name, choice.DisableParallelToolUse)
}

func (c *ToolBridgeConfig) setToolChoice(choiceType, name string, disableParallel bool) error {
	c.AllowParallel = !disableParallel
	switch strings.ToLower(strings.TrimSpace(choiceType)) {
	case "", "auto":
		c.ChoiceMode = ToolChoiceAuto
	case "none":
		c.ChoiceMode = ToolChoiceNone
	case "any", "required":
		c.ChoiceMode = ToolChoiceRequired
	case "tool", "function":
		name = strings.TrimSpace(name)
		if name == "" {
			return errors.New("claude Web specific tool_choice requires a name")
		}
		c.ChoiceMode = ToolChoiceSpecific
		c.ForcedTool = name
	default:
		return fmt.Errorf("unsupported claude Web tool_choice type %q", choiceType)
	}
	return nil
}

func (c *ToolBridgeConfig) Enabled() bool {
	return c != nil && len(c.Tools) > 0 && c.ChoiceMode != ToolChoiceNone
}

func (c *ToolBridgeConfig) RequiresTool() bool {
	return c != nil && (c.ChoiceMode == ToolChoiceRequired || c.ChoiceMode == ToolChoiceSpecific)
}

func (c *ToolBridgeConfig) Prompt() (prompt string, err error) {
	defer func() {
		if err != nil {
			err = &ToolBridgeRequestError{err: err}
		}
	}()
	if !c.Enabled() {
		return "", nil
	}
	toolsText, err := c.formatToolsForPrompt()
	if err != nil {
		return "", err
	}
	if len(toolsText) > maxToolBridgePrompt {
		return "", fmt.Errorf("claude Web tool definitions exceed %d bytes", maxToolBridgePrompt)
	}

	toolBlock := `<tool_calls>[{"name":"ToolName","arguments":{...}}]</tool_calls>`
	if !c.AllowParallel {
		toolBlock = `<tool_call>{"name":"ToolName","arguments":{...}}</tool_call>`
	}
	terminal := "either <tool_calls> or <final_answer>"
	if !c.AllowParallel {
		terminal = "either <tool_call> or <final_answer>"
	}
	required := ""
	if c.RequiresTool() {
		required = "\n- You must call one available tool in this response. Do not return <final_answer>."
	}

	return fmt.Sprintf(`You are a tool-capable assistant.

You must respond using only the following XML-like tags:
- <think>...</think>
- %s
- <final_answer>...</final_answer>

Rules:
- You may output one or more <think> blocks.
- You must then output exactly one terminal block: %s.
- Do not output any text outside these tags.
- In the tool call block, the content must be valid JSON with "name" and "arguments" fields.
- Tool arguments must be one JSON object matching the selected input schema.
- After the terminal closing tag, stop immediately.
- Never generate tool results or a second terminal block. Tool results are supplied in the next turn.%s

---

## Available tools

%s
`, toolBlock, terminal, required, toolsText), nil
}

func (c *ToolBridgeConfig) formatToolsForPrompt() (string, error) {
	var builder strings.Builder
	for index, tool := range c.Tools {
		if index > 0 {
			builder.WriteString("\n\n")
		}
		schema := bytes.TrimSpace(tool.InputSchema)
		if len(schema) == 0 {
			schema = []byte(`{"type":"object","properties":{}}`)
		}
		if !json.Valid(schema) {
			return "", fmt.Errorf("encode claude Web tool %q schema", tool.Name)
		}
		fmt.Fprintf(&builder, "Name: %s\nDescription: %s\nInput schema: %s", tool.Name, tool.Description, schema)
	}
	return builder.String(), nil
}

// ToolBridgeParser incrementally removes tagged protocol wrappers while allowing
// normal final answers to keep streaming. Tool-call JSON is buffered until it can
// be validated and emitted as structured Anthropic content blocks.
type ToolBridgeParser struct {
	config  *ToolBridgeConfig
	mode    string
	pending string
	done    bool
}

func NewToolBridgeParser(config *ToolBridgeConfig) *ToolBridgeParser {
	return &ToolBridgeParser{config: config, mode: "detect"}
}

func (p *ToolBridgeParser) Feed(fragment string) (string, []ToolCall, error) {
	if p == nil || fragment == "" {
		return "", nil, nil
	}
	if p.done {
		if strings.TrimSpace(fragment) != "" {
			return "", nil, errors.New("claude Web returned content after the tagged terminal block")
		}
		return "", nil, nil
	}
	p.pending += fragment
	return p.drain(false)
}

func (p *ToolBridgeParser) Finish() (string, []ToolCall, error) {
	if p == nil {
		return "", nil, nil
	}
	if p.done {
		return "", nil, nil
	}
	return p.drain(true)
}

func (p *ToolBridgeParser) drain(final bool) (string, []ToolCall, error) {
	var output strings.Builder
	for {
		switch p.mode {
		case "detect":
			trimmed := strings.TrimLeft(p.pending, " \t\r\n")
			if trimmed == "" && !final {
				return output.String(), nil, nil
			}
			matched, partial := matchToolBridgeOpeningTag(trimmed)
			if matched != "" {
				p.pending = strings.TrimPrefix(trimmed, matched)
				switch matched {
				case toolBridgeThinkingOpen:
					p.mode = "thinking"
				case toolBridgeCallsOpen:
					p.mode = "tool_calls"
				case toolBridgeCallOpen:
					p.mode = "tool_call"
				case toolBridgeFinalOpen:
					p.mode = "final"
				case legacyToolBridgeCallsOpen:
					p.mode = "legacy_tool_calls"
				case legacyToolBridgeCallOpen:
					p.mode = "legacy_tool_call"
				case legacyToolBridgeFinalOpen:
					p.mode = "legacy_final"
				}
				continue
			}
			if partial && !final {
				return output.String(), nil, nil
			}
			if p.config != nil && p.config.RequiresTool() {
				return "", nil, errors.New("claude Web did not return the required tagged tool call")
			}
			p.mode = "passthrough"
			continue

		case "passthrough":
			output.WriteString(p.pending)
			p.pending = ""
			if final {
				p.done = true
			}
			return output.String(), nil, nil

		case "thinking":
			if index := strings.Index(p.pending, toolBridgeThinkingClose); index >= 0 {
				p.pending = p.pending[index+len(toolBridgeThinkingClose):]
				p.mode = "detect"
				continue
			}
			if final {
				return "", nil, fmt.Errorf("claude Web response is missing %s", toolBridgeThinkingClose)
			}
			return output.String(), nil, nil

		case "final", "legacy_final":
			if p.config != nil && p.config.RequiresTool() {
				return "", nil, errors.New("claude Web returned a final answer when a tool call was required")
			}
			closeTag := toolBridgeFinalClose
			if p.mode == "legacy_final" {
				closeTag = legacyToolBridgeFinalClose
			}
			if index := strings.Index(p.pending, closeTag); index >= 0 {
				output.WriteString(p.pending[:index])
				trailing := p.pending[index+len(closeTag):]
				if strings.TrimSpace(trailing) != "" {
					return "", nil, errors.New("claude Web returned content after the tagged final answer")
				}
				p.pending = ""
				p.done = true
				return output.String(), nil, nil
			}
			if final {
				output.WriteString(p.pending)
				p.pending = ""
				p.done = true
				return output.String(), nil, nil
			}
			keep := len(closeTag) - 1
			if len(p.pending) <= keep {
				return output.String(), nil, nil
			}
			output.WriteString(p.pending[:len(p.pending)-keep])
			p.pending = p.pending[len(p.pending)-keep:]
			return output.String(), nil, nil

		case "tool_calls", "tool_call", "legacy_tool_calls", "legacy_tool_call":
			if len(p.pending) > maxToolBridgeArguments {
				return "", nil, errors.New("claude Web tool arguments are too large")
			}
			closeTag := toolBridgeCallsClose
			if p.mode == "tool_call" {
				closeTag = toolBridgeCallClose
			}
			if p.mode == "legacy_tool_calls" {
				closeTag = legacyToolBridgeCallsClose
			}
			if p.mode == "legacy_tool_call" {
				closeTag = legacyToolBridgeCallClose
			}
			index := strings.Index(p.pending, closeTag)
			if index < 0 {
				if final {
					return "", nil, fmt.Errorf("claude Web response is missing %s", closeTag)
				}
				return output.String(), nil, nil
			}
			rawCalls := strings.TrimSpace(p.pending[:index])
			trailing := p.pending[index+len(closeTag):]
			if strings.TrimSpace(trailing) != "" {
				return "", nil, errors.New("claude Web returned content after the tagged tool call")
			}
			calls, err := p.parseCalls(rawCalls, p.mode == "tool_call" || p.mode == "legacy_tool_call")
			if err != nil {
				return "", nil, err
			}
			p.pending = ""
			p.done = true
			return output.String(), calls, nil

		default:
			return "", nil, fmt.Errorf("unknown claude Web tool bridge parser mode %q", p.mode)
		}
	}
}

func matchToolBridgeOpeningTag(value string) (matched string, partial bool) {
	for _, tag := range []string{
		toolBridgeThinkingOpen,
		toolBridgeCallsOpen,
		toolBridgeCallOpen,
		toolBridgeFinalOpen,
		legacyToolBridgeCallsOpen,
		legacyToolBridgeCallOpen,
		legacyToolBridgeFinalOpen,
	} {
		if strings.HasPrefix(value, tag) {
			return tag, false
		}
		if strings.HasPrefix(tag, value) {
			partial = true
		}
	}
	return "", partial
}

func (p *ToolBridgeParser) parseCalls(raw string, single bool) ([]ToolCall, error) {
	var tagged []taggedToolCall
	if single {
		var call taggedToolCall
		if err := json.Unmarshal([]byte(raw), &call); err != nil {
			return nil, fmt.Errorf("decode claude Web tool call: %w", err)
		}
		tagged = []taggedToolCall{call}
	} else if err := json.Unmarshal([]byte(raw), &tagged); err != nil {
		return nil, fmt.Errorf("decode claude Web tool calls: %w", err)
	}
	if len(tagged) == 0 {
		return nil, errors.New("claude Web returned an empty tool call list")
	}
	if p.config != nil && !p.config.AllowParallel && len(tagged) > 1 {
		return nil, errors.New("claude Web returned parallel tool calls when they were disabled")
	}

	calls := make([]ToolCall, 0, len(tagged))
	for _, call := range tagged {
		call.Name = strings.TrimSpace(call.Name)
		if call.Name == "" {
			return nil, errors.New("claude Web returned a tool call without a name")
		}
		if p.config != nil {
			if _, ok := p.config.toolNames[call.Name]; !ok {
				return nil, fmt.Errorf("claude Web returned unknown tool %q", call.Name)
			}
			if p.config.ChoiceMode == ToolChoiceSpecific && call.Name != p.config.ForcedTool {
				return nil, fmt.Errorf("claude Web returned tool %q instead of required tool %q", call.Name, p.config.ForcedTool)
			}
		}
		arguments := bytes.TrimSpace(call.Arguments)
		if len(arguments) == 0 || string(arguments) == "null" {
			arguments = []byte("{}")
		}
		var object map[string]json.RawMessage
		if json.Unmarshal(arguments, &object) != nil || object == nil {
			return nil, fmt.Errorf("claude Web tool %q arguments must be a JSON object", call.Name)
		}
		canonical, err := json.Marshal(object)
		if err != nil {
			return nil, fmt.Errorf("encode claude Web tool %q arguments: %w", call.Name, err)
		}
		calls = append(calls, ToolCall{Name: call.Name, Arguments: canonical})
	}
	return calls, nil
}
