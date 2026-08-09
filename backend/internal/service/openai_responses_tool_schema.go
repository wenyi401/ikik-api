package service

import (
	"fmt"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const (
	openAIResponsesToolSchemaMaxDepth     = 4
	openAIResponsesToolSchemaFallbackType = "object"
)

// sanitizeOpenAIResponsesToolParameterTypes repairs only an explicit null
// JSON Schema type, which OpenAI rejects for function parameters. A missing
// type remains valid and must keep its client-provided semantics.
func sanitizeOpenAIResponsesToolParameterTypes(body []byte) ([]byte, bool, error) {
	if len(body) == 0 {
		return body, false, nil
	}

	paths := make([]string, 0, 2)
	collectOpenAIResponsesToolSchemaNullTypePaths(gjson.GetBytes(body, "tools"), "tools", 0, &paths)
	if input := gjson.GetBytes(body, "input"); input.IsArray() {
		index := 0
		input.ForEach(func(_, item gjson.Result) bool {
			if item.IsObject() {
				collectOpenAIResponsesToolSchemaNullTypePaths(item.Get("tools"), fmt.Sprintf("input.%d.tools", index), 0, &paths)
			}
			index++
			return true
		})
	}
	if len(paths) == 0 {
		return body, false, nil
	}

	sanitized := body
	for _, path := range paths {
		next, err := sjson.SetBytes(sanitized, path, openAIResponsesToolSchemaFallbackType)
		if err != nil {
			return body, false, fmt.Errorf("normalize %s: %w", path, err)
		}
		sanitized = next
	}
	return sanitized, true, nil
}

func collectOpenAIResponsesToolSchemaNullTypePaths(tools gjson.Result, basePath string, depth int, paths *[]string) {
	if depth > openAIResponsesToolSchemaMaxDepth || !tools.IsArray() {
		return
	}

	index := 0
	tools.ForEach(func(_, tool gjson.Result) bool {
		toolPath := fmt.Sprintf("%s.%d", basePath, index)
		index++
		if !tool.IsObject() {
			return true
		}
		for _, suffix := range []string{"parameters", "function.parameters"} {
			params := tool.Get(suffix)
			if params.IsObject() && params.Get("type").Exists() && params.Get("type").Type == gjson.Null {
				*paths = append(*paths, toolPath+"."+suffix+".type")
			}
		}
		collectOpenAIResponsesToolSchemaNullTypePaths(tool.Get("tools"), toolPath+".tools", depth+1, paths)
		return true
	})
}
