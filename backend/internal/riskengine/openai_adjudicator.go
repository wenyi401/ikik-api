package riskengine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const maxAdjudicatorResponseBytes = 1024 * 1024

type OpenAIAdjudicatorConfig struct {
	Endpoint string
	Model    string
	Token    string
}

type OpenAIAdjudicator struct {
	endpoint string
	model    string
	token    string
	client   *http.Client
}

type openAIAdjudicatorRequest struct {
	Model          string                       `json:"model"`
	Messages       []openAIAdjudicatorMessage   `json:"messages"`
	Temperature    float64                      `json:"temperature"`
	Stream         bool                         `json:"stream"`
	ResponseFormat *openAIAdjudicatorJSONFormat `json:"response_format,omitempty"`
}

type openAIAdjudicatorMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIAdjudicatorJSONFormat struct {
	Type string `json:"type"`
}

type openAIAdjudicatorResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func NewOpenAIAdjudicator(config OpenAIAdjudicatorConfig, client *http.Client) (*OpenAIAdjudicator, error) {
	endpoint, err := normalizeAdjudicatorEndpoint(config.Endpoint)
	if err != nil {
		return nil, err
	}
	model := strings.TrimSpace(config.Model)
	if model == "" {
		return nil, errors.New("risk adjudicator model is required")
	}
	if client == nil {
		client = http.DefaultClient
	}
	return &OpenAIAdjudicator{
		endpoint: endpoint, model: model, token: strings.TrimSpace(config.Token), client: client,
	}, nil
}

func (a *OpenAIAdjudicator) Adjudicate(ctx context.Context, input Input, _ Candidate) (Adjudication, error) {
	if a == nil || a.client == nil {
		return Adjudication{}, errors.New("risk adjudicator unavailable")
	}
	payload, err := json.Marshal(openAIAdjudicatorRequest{
		Model: a.model,
		Messages: []openAIAdjudicatorMessage{
			{Role: "system", Content: AdjudicatorSystemInstruction()},
			{Role: "user", Content: AdjudicatorUserContent(input.Text)},
		},
		Temperature: 0, Stream: false,
		ResponseFormat: &openAIAdjudicatorJSONFormat{Type: "json_object"},
	})
	if err != nil {
		return Adjudication{}, fmt.Errorf("marshal risk adjudicator request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, a.endpoint, bytes.NewReader(payload))
	if err != nil {
		return Adjudication{}, fmt.Errorf("build risk adjudicator request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	if a.token != "" {
		request.Header.Set("Authorization", "Bearer "+a.token)
	}
	response, err := a.client.Do(request)
	if err != nil {
		return Adjudication{}, fmt.Errorf("call risk adjudicator: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
		return Adjudication{}, fmt.Errorf("risk adjudicator status %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxAdjudicatorResponseBytes+1))
	if err != nil {
		return Adjudication{}, fmt.Errorf("read risk adjudicator response: %w", err)
	}
	if len(body) > maxAdjudicatorResponseBytes {
		return Adjudication{}, errors.New("risk adjudicator response exceeds size limit")
	}
	var envelope openAIAdjudicatorResponse
	if err := json.Unmarshal(body, &envelope); err != nil {
		return Adjudication{}, fmt.Errorf("decode risk adjudicator response envelope: %w", err)
	}
	if len(envelope.Choices) == 0 || strings.TrimSpace(envelope.Choices[0].Message.Content) == "" {
		return Adjudication{}, errors.New("risk adjudicator returned no decision")
	}
	decision, err := ParseAdjudication(input.Text, []byte(envelope.Choices[0].Message.Content))
	if err != nil {
		return Adjudication{}, err
	}
	decision.Model = a.model
	return decision, nil
}

func normalizeAdjudicatorEndpoint(value string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed == nil || parsed.Host == "" {
		return "", errors.New("risk adjudicator endpoint is invalid")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("risk adjudicator endpoint must use http or https")
	}
	if parsed.User != nil || parsed.Fragment != "" || parsed.RawQuery != "" {
		return "", errors.New("risk adjudicator endpoint cannot include credentials, query, or fragment")
	}
	path := strings.TrimRight(parsed.Path, "/")
	if !strings.HasSuffix(path, "/chat/completions") {
		if strings.HasSuffix(path, "/v1") {
			path += "/chat/completions"
		} else {
			path += "/v1/chat/completions"
		}
	}
	parsed.Path = path
	return parsed.String(), nil
}
