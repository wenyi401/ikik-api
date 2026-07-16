package claudeweb

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"ikik-api/internal/pkg/anthropictokenizer"

	"github.com/google/uuid"
)

type StreamUsage struct {
	InputTokens  int
	OutputTokens int
}

type webStreamEvent struct {
	Type  string          `json:"type"`
	Index *int            `json:"index,omitempty"`
	Delta json.RawMessage `json:"delta,omitempty"`
	Error struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type webStreamDelta struct {
	Type        string `json:"type"`
	Text        string `json:"text,omitempty"`
	Thinking    string `json:"thinking,omitempty"`
	PartialJSON string `json:"partial_json,omitempty"`
	StopReason  string `json:"stop_reason,omitempty"`
}

type StreamOptions struct {
	Model       string
	InputTokens int
	ToolBridge  *ToolBridgeConfig
}

func ConvertStream(ctx context.Context, source io.Reader, destination io.Writer, model string, inputTokens int) (StreamUsage, error) {
	return ConvertStreamWithOptions(ctx, source, destination, StreamOptions{Model: model, InputTokens: inputTokens})
}

func ConvertStreamWithOptions(ctx context.Context, source io.Reader, destination io.Writer, options StreamOptions) (StreamUsage, error) {
	usage := StreamUsage{InputTokens: options.InputTokens}
	messageID := "msg_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	messageStart := map[string]any{
		"type": "message_start",
		"message": map[string]any{
			"id":            messageID,
			"type":          "message",
			"role":          "assistant",
			"content":       []any{},
			"model":         options.Model,
			"stop_reason":   nil,
			"stop_sequence": nil,
			"usage": map[string]int{
				"input_tokens":                options.InputTokens,
				"output_tokens":               0,
				"cache_creation_input_tokens": 0,
				"cache_read_input_tokens":     0,
			},
		},
	}
	if err := writeSSE(destination, "message_start", messageStart); err != nil {
		return usage, err
	}

	scanner := bufio.NewScanner(source)
	scanner.Buffer(make([]byte, 0, 64*1024), 16<<20)
	startedBlocks := make(map[int]string)
	outputText := strings.Builder{}
	stopReason := ""
	bridge := options.ToolBridge
	var bridgeParser *ToolBridgeParser
	if bridge != nil && bridge.Enabled() {
		bridgeParser = NewToolBridgeParser(bridge)
	}
	bridgeIndex := -1
	bridgeTextStarted := false
	bridgeTextStopped := false
	bridgeToolCallsEmitted := false
	nextContentIndex := 0

	writeBridgeText := func(text string) error {
		if text == "" {
			return nil
		}
		if bridgeToolCallsEmitted {
			return errors.New("claude Web returned text after a tool call")
		}
		if bridgeIndex < 0 {
			bridgeIndex = nextContentIndex
		}
		if !bridgeTextStarted {
			start := map[string]any{
				"type":  "content_block_start",
				"index": bridgeIndex,
				"content_block": map[string]any{
					"type": "text",
					"text": "",
				},
			}
			if err := writeSSE(destination, "content_block_start", start); err != nil {
				return err
			}
			bridgeTextStarted = true
			nextContentIndex = maxStreamIndex(nextContentIndex, bridgeIndex+1)
		}
		return writeSSE(destination, "content_block_delta", map[string]any{
			"type":  "content_block_delta",
			"index": bridgeIndex,
			"delta": map[string]any{"type": "text_delta", "text": text},
		})
	}

	stopBridgeText := func() error {
		if !bridgeTextStarted || bridgeTextStopped {
			return nil
		}
		bridgeTextStopped = true
		return writeSSE(destination, "content_block_stop", map[string]any{
			"type":  "content_block_stop",
			"index": bridgeIndex,
		})
	}

	emitBridgeCalls := func(calls []ToolCall) error {
		if len(calls) == 0 {
			return nil
		}
		if err := stopBridgeText(); err != nil {
			return err
		}
		for callIndex, call := range calls {
			index := nextContentIndex
			if callIndex == 0 && !bridgeTextStarted && bridgeIndex >= 0 {
				index = bridgeIndex
			}
			nextContentIndex = maxStreamIndex(nextContentIndex, index+1)
			callID := "toolu_" + strings.ReplaceAll(uuid.NewString(), "-", "")
			if err := writeSSE(destination, "content_block_start", map[string]any{
				"type":  "content_block_start",
				"index": index,
				"content_block": map[string]any{
					"type":  "tool_use",
					"id":    callID,
					"name":  call.Name,
					"input": map[string]any{},
				},
			}); err != nil {
				return err
			}
			if err := writeSSE(destination, "content_block_delta", map[string]any{
				"type":  "content_block_delta",
				"index": index,
				"delta": map[string]any{
					"type":         "input_json_delta",
					"partial_json": string(call.Arguments),
				},
			}); err != nil {
				return err
			}
			if err := writeSSE(destination, "content_block_stop", map[string]any{
				"type":  "content_block_stop",
				"index": index,
			}); err != nil {
				return err
			}
		}
		bridgeToolCallsEmitted = true
		stopReason = "tool_use"
		return nil
	}

	processEvent := func(eventType, payload string) error {
		payload = strings.TrimSpace(payload)
		if payload == "" || payload == "[DONE]" {
			return nil
		}

		rawPayload, event, ok := normalizeWebStreamEvent(eventType, payload)
		if !ok {
			return nil
		}
		switch event.Type {
		case "message_start", "message_stop", "ping":
			return nil
		case "message_delta":
			var delta webStreamDelta
			if !bridgeToolCallsEmitted && json.Unmarshal(event.Delta, &delta) == nil && strings.TrimSpace(delta.StopReason) != "" {
				stopReason = strings.TrimSpace(delta.StopReason)
			}
			return nil
		case "error":
			message := strings.TrimSpace(event.Error.Message)
			if message == "" {
				message = "claude web stream returned an error"
			}
			return fmt.Errorf("%s", message)
		case "content_block_start":
			index := eventIndex(event.Index)
			blockType := contentBlockType(string(rawPayload))
			startedBlocks[index] = blockType
			nextContentIndex = maxStreamIndex(nextContentIndex, index+1)
			if bridgeParser != nil && blockType == "text" {
				if bridgeIndex < 0 {
					bridgeIndex = index
				}
				return nil
			}
			if blockType == "tool_use" {
				stopReason = "tool_use"
			}
			return writeRawSSE(destination, event.Type, rawPayload)
		case "content_block_delta":
			index := eventIndex(event.Index)
			var delta webStreamDelta
			_ = json.Unmarshal(event.Delta, &delta)
			for _, text := range []string{delta.Text, delta.Thinking, delta.PartialJSON} {
				if _, err := outputText.WriteString(text); err != nil {
					return err
				}
			}
			if bridgeParser != nil && delta.Type == "text_delta" {
				if bridgeIndex < 0 {
					bridgeIndex = index
				}
				text, calls, err := bridgeParser.Feed(delta.Text)
				if err != nil {
					return err
				}
				if err := writeBridgeText(text); err != nil {
					return err
				}
				return emitBridgeCalls(calls)
			}
			if _, ok := startedBlocks[index]; !ok {
				blockType := "text"
				if delta.Type == "thinking_delta" {
					blockType = "thinking"
				}
				startedBlocks[index] = blockType
				contentBlock := map[string]any{
					"type": blockType,
					"text": "",
				}
				start := map[string]any{
					"type":          "content_block_start",
					"index":         index,
					"content_block": contentBlock,
				}
				if blockType == "thinking" {
					contentBlock["thinking"] = ""
					delete(contentBlock, "text")
				}
				if err := writeSSE(destination, "content_block_start", start); err != nil {
					return err
				}
				nextContentIndex = maxStreamIndex(nextContentIndex, index+1)
			}
			return writeRawSSE(destination, event.Type, rawPayload)
		case "content_block_stop":
			index := eventIndex(event.Index)
			if bridgeParser != nil && startedBlocks[index] == "text" {
				return nil
			}
			return writeRawSSE(destination, event.Type, rawPayload)
		default:
			if event.Type != "" {
				return writeRawSSE(destination, event.Type, rawPayload)
			}
			return nil
		}
	}

	var eventType string
	var dataLines []string
	flushEvent := func() error {
		err := processEvent(eventType, strings.Join(dataLines, "\n"))
		eventType = ""
		dataLines = dataLines[:0]
		return err
	}
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return usage, ctx.Err()
		default:
		}

		line := scanner.Text()
		if line == "" {
			if err := flushEvent(); err != nil {
				return usage, err
			}
			continue
		}
		if strings.HasPrefix(line, "event:") {
			eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if err := scanner.Err(); err != nil {
		return usage, fmt.Errorf("read claude web stream: %w", err)
	}
	if err := flushEvent(); err != nil {
		return usage, err
	}
	if bridgeParser != nil {
		text, calls, err := bridgeParser.Finish()
		if err != nil {
			return usage, err
		}
		if err := writeBridgeText(text); err != nil {
			return usage, err
		}
		if err := emitBridgeCalls(calls); err != nil {
			return usage, err
		}
		if err := stopBridgeText(); err != nil {
			return usage, err
		}
	}

	usage.OutputTokens = anthropictokenizer.CountTokens(outputText.String())
	if stopReason == "" {
		stopReason = "end_turn"
	}
	messageDelta := map[string]any{
		"type": "message_delta",
		"delta": map[string]any{
			"stop_reason":   stopReason,
			"stop_sequence": nil,
		},
		"usage": map[string]int{
			"output_tokens": usage.OutputTokens,
		},
	}
	if err := writeSSE(destination, "message_delta", messageDelta); err != nil {
		return usage, err
	}
	if err := writeSSE(destination, "message_stop", map[string]string{"type": "message_stop"}); err != nil {
		return usage, err
	}
	return usage, nil
}

func maxStreamIndex(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func normalizeWebStreamEvent(eventType, payload string) ([]byte, webStreamEvent, bool) {
	rawPayload := []byte(payload)
	var event webStreamEvent
	if json.Unmarshal(rawPayload, &event) != nil {
		return nil, event, false
	}
	event.Type = strings.TrimSpace(event.Type)
	if event.Type == "" {
		event.Type = strings.TrimSpace(eventType)
		if event.Type == "" {
			return nil, event, false
		}
		var value map[string]any
		if json.Unmarshal(rawPayload, &value) != nil {
			return nil, event, false
		}
		value["type"] = event.Type
		normalized, err := json.Marshal(value)
		if err != nil {
			return nil, event, false
		}
		rawPayload = normalized
	}
	return rawPayload, event, true
}

func eventIndex(index *int) int {
	if index == nil || *index < 0 {
		return 0
	}
	return *index
}

func contentBlockType(payload string) string {
	var value struct {
		ContentBlock struct {
			Type string `json:"type"`
		} `json:"content_block"`
	}
	if json.Unmarshal([]byte(payload), &value) != nil {
		return ""
	}
	return strings.TrimSpace(value.ContentBlock.Type)
}

func writeSSE(destination io.Writer, eventType string, value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return writeRawSSE(destination, eventType, payload)
}

func writeRawSSE(destination io.Writer, eventType string, payload []byte) error {
	if _, err := fmt.Fprintf(destination, "event: %s\n", eventType); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(destination, "data: %s\n\n", payload); err != nil {
		return err
	}
	return nil
}
