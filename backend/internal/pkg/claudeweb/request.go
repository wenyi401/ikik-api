package claudeweb

import (
	"encoding/json"
	"strings"

	"github.com/google/uuid"
)

type CompletionOptions struct {
	Model             string
	Prompt            string
	Effort            string
	ThinkingMode      string
	Timezone          string
	Locale            string
	ConversationID    string
	ParentMessageUUID string
	Persistent        bool
	DisableWebTools   bool
}

type completionRequest struct {
	Prompt                   string                    `json:"prompt"`
	ParentMessageUUID        string                    `json:"parent_message_uuid,omitempty"`
	Timezone                 string                    `json:"timezone"`
	Locale                   string                    `json:"locale"`
	Model                    string                    `json:"model"`
	Effort                   string                    `json:"effort,omitempty"`
	ThinkingMode             string                    `json:"thinking_mode,omitempty"`
	Tools                    json.RawMessage           `json:"tools,omitempty"`
	TurnMessageUUIDs         *turnMessageUUIDs         `json:"turn_message_uuids,omitempty"`
	Attachments              []map[string]any          `json:"attachments"`
	Files                    []map[string]any          `json:"files"`
	SyncSources              []any                     `json:"sync_sources"`
	RenderingMode            string                    `json:"rendering_mode"`
	CreateConversationParams *createConversationParams `json:"create_conversation_params,omitempty"`
}

type turnMessageUUIDs struct {
	HumanMessageUUID     string `json:"human_message_uuid"`
	AssistantMessageUUID string `json:"assistant_message_uuid"`
}

type createConversationParams struct {
	Name                           string `json:"name"`
	Model                          string `json:"model"`
	IncludeConversationPreferences bool   `json:"include_conversation_preferences"`
	PaprikaMode                    any    `json:"paprika_mode"`
	CompassMode                    any    `json:"compass_mode"`
	ToolSearchMode                 string `json:"tool_search_mode"`
	IsTemporary                    bool   `json:"is_temporary"`
	EnabledImagine                 bool   `json:"enabled_imagine"`
}

func normalizeCompletionOptions(options CompletionOptions) CompletionOptions {
	options.Model = strings.TrimSpace(options.Model)
	if options.Model == "" {
		options.Model = DefaultModel
	}
	options.Effort = strings.ToLower(strings.TrimSpace(options.Effort))
	switch options.Effort {
	case "low", "medium", "high", "xhigh", "max":
	default:
		options.Effort = "medium"
	}
	options.ThinkingMode = strings.ToLower(strings.TrimSpace(options.ThinkingMode))
	if options.ThinkingMode != "none" {
		options.ThinkingMode = "auto"
	}
	options.Timezone = strings.TrimSpace(options.Timezone)
	if options.Timezone == "" {
		options.Timezone = DefaultTimezone
	}
	options.Locale = strings.TrimSpace(options.Locale)
	if options.Locale == "" {
		options.Locale = DefaultLocale
	}
	return options
}

func buildCompletionRequest(options CompletionOptions) (*completionRequest, string, string) {
	options = normalizeCompletionOptions(options)
	humanUUID := uuid.NewString()
	assistantUUID := uuid.NewString()
	parentUUID := strings.TrimSpace(options.ParentMessageUUID)
	if parentUUID == "" {
		parentUUID = uuid.NewString()
	}

	tools := WebTools()
	if options.DisableWebTools {
		// Client-defined tools use the prompt bridge. Keeping claude.ai's own
		// server tools enabled would expose calls that the API client cannot run.
		tools = json.RawMessage(`[]`)
	}

	return &completionRequest{
		Prompt:            options.Prompt,
		ParentMessageUUID: parentUUID,
		Timezone:          options.Timezone,
		Locale:            options.Locale,
		Model:             options.Model,
		Effort:            options.Effort,
		ThinkingMode:      options.ThinkingMode,
		Tools:             tools,
		TurnMessageUUIDs: &turnMessageUUIDs{
			HumanMessageUUID:     humanUUID,
			AssistantMessageUUID: assistantUUID,
		},
		Attachments:   []map[string]any{},
		Files:         []map[string]any{},
		SyncSources:   []any{},
		RenderingMode: "messages",
		CreateConversationParams: &createConversationParams{
			Name:                           "",
			Model:                          options.Model,
			IncludeConversationPreferences: true,
			PaprikaMode:                    nil,
			CompassMode:                    nil,
			ToolSearchMode:                 "auto",
			IsTemporary:                    false,
			EnabledImagine:                 true,
		},
	}, humanUUID, assistantUUID
}
