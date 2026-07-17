package service

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/tidwall/gjson"
)

// contentSessionSeedPrefix prevents collisions between content-derived seeds
// and explicit session IDs (e.g. "sess-xxx" or "compat_cc_xxx").
const contentSessionSeedPrefix = "compat_cs_"

// deriveOpenAIContentSessionSeed builds a stable session seed from an
// OpenAI-format request body. Only fields constant across conversation turns
// are included: model, tools/functions definitions, system/developer prompts,
// instructions (Responses API), and the first user message.
// Supports both Chat Completions (messages) and Responses API (input).
func deriveOpenAIContentSessionSeed(body []byte) string {
	if len(body) == 0 {
		return ""
	}

	const (
		modelField = iota
		toolsField
		functionsField
		instructionsField
		messagesField
		inputField
		contentSessionSeedFieldCount
		allContentSessionSeedFields = 1<<contentSessionSeedFieldCount - 1
	)
	var fields [contentSessionSeedFieldCount]gjson.Result
	var seen uint8
	// Match gjson.GetBytes by starting at the first root container, even when
	// malformed input has a non-JSON prefix.
	root := body
	for i := 0; i < len(body); i++ {
		switch body[i] {
		case '{':
			root = body[i:]
			goto scanRoot
		case '[':
			return ""
		}
	}
	return ""

scanRoot:
	nextKeyOffset := 1
	parseRawJSONView(root).ForEach(func(key, value gjson.Result) bool {
		if key.Index < nextKeyOffset || key.Index > len(root) {
			return false
		}
		// Result.ForEach can continue after the root '}' on malformed input.
		// The separator range excludes braces inside the preceding parsed value.
		if bytes.IndexByte(root[nextKeyOffset:key.Index], '}') >= 0 {
			return false
		}
		nextKeyOffset = value.Index + len(value.Raw)

		field := -1
		switch key.Str {
		case "model":
			field = modelField
		case "tools":
			field = toolsField
		case "functions":
			field = functionsField
		case "instructions":
			field = instructionsField
		case "messages":
			field = messagesField
		case "input":
			field = inputField
		}
		if field < 0 {
			return true
		}
		mask := uint8(1 << field)
		if seen&mask == 0 {
			fields[field] = value
			seen |= mask
		}
		return seen != allContentSessionSeedFields
	})

	var b strings.Builder

	if model := fields[modelField].String(); model != "" {
		_, _ = b.WriteString("model=")
		_, _ = b.WriteString(model)
	}

	if tools := fields[toolsField]; tools.Exists() && tools.IsArray() && tools.Raw != "[]" {
		_, _ = b.WriteString("|tools=")
		_, _ = b.WriteString(normalizeCompatSeedJSON(json.RawMessage(tools.Raw)))
	}

	if funcs := fields[functionsField]; funcs.Exists() && funcs.IsArray() && funcs.Raw != "[]" {
		_, _ = b.WriteString("|functions=")
		_, _ = b.WriteString(normalizeCompatSeedJSON(json.RawMessage(funcs.Raw)))
	}

	if instr := fields[instructionsField].String(); instr != "" {
		_, _ = b.WriteString("|instructions=")
		_, _ = b.WriteString(instr)
	}

	firstUserCaptured := false

	msgs := fields[messagesField]
	if msgs.Exists() && msgs.IsArray() {
		msgs.ForEach(func(_, msg gjson.Result) bool {
			role := msg.Get("role").String()
			switch role {
			case "system", "developer":
				_, _ = b.WriteString("|system=")
				if c := msg.Get("content"); c.Exists() {
					_, _ = b.WriteString(normalizeCompatSeedJSON(json.RawMessage(c.Raw)))
				}
			case "user":
				if !firstUserCaptured {
					_, _ = b.WriteString("|first_user=")
					if c := msg.Get("content"); c.Exists() {
						_, _ = b.WriteString(normalizeCompatSeedJSON(json.RawMessage(c.Raw)))
					}
					firstUserCaptured = true
				}
			}
			return true
		})
	} else if inp := fields[inputField]; inp.Exists() {
		if inp.Type == gjson.String {
			_, _ = b.WriteString("|input=")
			_, _ = b.WriteString(inp.String())
		} else if inp.IsArray() {
			inp.ForEach(func(_, item gjson.Result) bool {
				role := item.Get("role").String()
				switch role {
				case "system", "developer":
					_, _ = b.WriteString("|system=")
					if c := item.Get("content"); c.Exists() {
						_, _ = b.WriteString(normalizeCompatSeedJSON(json.RawMessage(c.Raw)))
					}
				case "user":
					if !firstUserCaptured {
						_, _ = b.WriteString("|first_user=")
						if c := item.Get("content"); c.Exists() {
							_, _ = b.WriteString(normalizeCompatSeedJSON(json.RawMessage(c.Raw)))
						}
						firstUserCaptured = true
					}
				}
				if !firstUserCaptured && item.Get("type").String() == "input_text" {
					_, _ = b.WriteString("|first_user=")
					if text := item.Get("text").String(); text != "" {
						_, _ = b.WriteString(text)
					}
					firstUserCaptured = true
				}
				return true
			})
		}
	}

	if b.Len() == 0 {
		return ""
	}
	return contentSessionSeedPrefix + b.String()
}
