package service

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const geminiImageOutputCounterKey = "gemini_image_output_counter"

// geminiImageOutputCounter keeps the largest image count observed in one
// upstream payload. Gemini SSE chunks can be cumulative, so summing chunks
// would charge the same image repeatedly.
type geminiImageOutputCounter struct {
	count int
}

func beginGeminiImageOutputObservation(c *gin.Context) *geminiImageOutputCounter {
	if c == nil {
		return nil
	}
	counter := &geminiImageOutputCounter{}
	c.Set(geminiImageOutputCounterKey, counter)
	return counter
}

func geminiImageOutputCounterFromContext(c *gin.Context) *geminiImageOutputCounter {
	if c == nil {
		return nil
	}
	value, ok := c.Get(geminiImageOutputCounterKey)
	if !ok {
		return nil
	}
	counter, _ := value.(*geminiImageOutputCounter)
	return counter
}

func observeGeminiImageOutputs(c *gin.Context, payload []byte) {
	counter := geminiImageOutputCounterFromContext(c)
	if counter == nil {
		return
	}
	if count := countGeminiInlineImageOutputs(payload); count > counter.count {
		counter.count = count
	}
}

func observedGeminiImageOutputs(c *gin.Context) int {
	counter := geminiImageOutputCounterFromContext(c)
	if counter == nil {
		return 0
	}
	return counter.count
}

func resolveGeminiImageCount(c *gin.Context, originalModel, mappedModel string) int {
	if observed := observedGeminiImageOutputs(c); observed > 0 {
		return observed
	}
	if isImageGenerationModel(originalModel) || isImageGenerationModel(mappedModel) {
		return 1
	}
	return 0
}

func countGeminiInlineImageOutputs(payload []byte) int {
	if len(payload) == 0 || !gjson.ValidBytes(payload) {
		return 0
	}
	count := 0
	gjson.GetBytes(payload, "candidates").ForEach(func(_, candidate gjson.Result) bool {
		candidate.Get("content.parts").ForEach(func(_, part gjson.Result) bool {
			if geminiPartIsInlineImage(part) {
				count++
			}
			return true
		})
		return true
	})
	return count
}

func geminiPartIsInlineImage(part gjson.Result) bool {
	inline := part.Get("inlineData")
	if !inline.Exists() {
		inline = part.Get("inline_data")
	}
	if !inline.Exists() {
		return false
	}

	mimeType := inline.Get("mimeType")
	if !mimeType.Exists() {
		mimeType = inline.Get("mime_type")
	}
	if !isGeminiInlineImageMIMEType(strings.ToLower(strings.TrimSpace(mimeType.String()))) {
		return false
	}
	return strings.TrimSpace(inline.Get("data").String()) != ""
}

func isGeminiInlineImageMIMEType(mimeType string) bool {
	return strings.HasPrefix(mimeType, "image/")
}
