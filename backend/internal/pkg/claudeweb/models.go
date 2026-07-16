package claudeweb

import (
	"fmt"
	"strings"
)

const (
	DefaultModel    = "claude-sonnet-5"
	DefaultTimezone = "Asia/Singapore"
	DefaultLocale   = "en-US"
)

var supportedModels = []string{
	"claude-fable-5",
	"claude-opus-4-8",
	"claude-haiku-4-5",
	"claude-opus-4-7",
	"claude-opus-4-6",
	"claude-opus-3",
	"claude-sonnet-4-6",
	"claude-sonnet-5",
}

type UnsupportedModelError struct {
	Model string
}

func (e *UnsupportedModelError) Error() string {
	if e == nil {
		return "unsupported Claude Web model"
	}
	return fmt.Sprintf("Claude Web model %q is not supported", e.Model)
}

func SupportedModels() []string {
	return append([]string(nil), supportedModels...)
}

func ValidateModel(model string) error {
	model = strings.TrimSpace(model)
	if model == "" {
		model = DefaultModel
	}
	for _, supported := range supportedModels {
		if model == supported {
			return nil
		}
	}
	return &UnsupportedModelError{Model: model}
}
