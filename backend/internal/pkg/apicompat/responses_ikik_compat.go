package apicompat

import (
	"encoding/json"
	"strings"
)

func (item *ResponsesInputItem) UnmarshalJSON(data []byte) error {
	type alias ResponsesInputItem
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	arguments := fields["arguments"]
	output := fields["output"]
	delete(fields, "arguments")
	delete(fields, "output")
	baseJSON, err := json.Marshal(fields)
	if err != nil {
		return err
	}
	var base alias
	if err := json.Unmarshal(baseJSON, &base); err != nil {
		return err
	}
	*item = ResponsesInputItem(base)
	item.Arguments = decodeFlexibleResponsesString(arguments, false)
	item.Output = decodeFlexibleResponsesString(output, true)
	return nil
}

func decodeFlexibleResponsesString(raw json.RawMessage, output bool) string {
	trimmed := json.RawMessage(strings.TrimSpace(string(raw)))
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return ""
	}
	var value string
	if err := json.Unmarshal(trimmed, &value); err == nil {
		return value
	}
	if output {
		return extractResponsesOutputText(trimmed)
	}
	if json.Valid(trimmed) {
		return string(trimmed)
	}
	return ""
}

func normalizeResponsesArguments(raw json.RawMessage) json.RawMessage {
	trimmed := json.RawMessage(strings.TrimSpace(string(raw)))
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return json.RawMessage("{}")
	}

	var encoded string
	if err := json.Unmarshal(trimmed, &encoded); err == nil {
		encoded = strings.TrimSpace(encoded)
		if encoded != "" && json.Valid([]byte(encoded)) {
			return json.RawMessage(encoded)
		}
		return json.RawMessage("{}")
	}
	if !json.Valid(trimmed) {
		return json.RawMessage("{}")
	}
	return trimmed
}

func extractResponsesOutputText(raw json.RawMessage) string {
	trimmed := json.RawMessage(strings.TrimSpace(string(raw)))
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return ""
	}

	var text string
	if err := json.Unmarshal(trimmed, &text); err == nil {
		return text
	}

	var parts []ResponsesContentPart
	if err := json.Unmarshal(trimmed, &parts); err == nil {
		texts := make([]string, 0, len(parts))
		for _, part := range parts {
			if part.Text != "" {
				texts = append(texts, part.Text)
			}
		}
		return strings.Join(texts, "\n\n")
	}
	return string(trimmed)
}
