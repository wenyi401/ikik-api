package service

import "testing"

func TestOpenAIImageOutputCounterDataArraySkipsNonImageObjects(t *testing.T) {
	sseWithNonImageData := `data: {"type":"response.completed","response":{"id":"resp_1","output":[{"id":"item_1","type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"Hello"}]}]},"data":[{"id":"not_an_image","status":"done"}]}

data: [DONE]`

	count := countOpenAIImageOutputsFromSSEBody(sseWithNonImageData)
	if count != 0 {
		t.Fatalf("expected 0 images for data array without image output, got %d", count)
	}

	sseWithImageData := `data: {"type":"response.completed","response":{"id":"resp_1","output":[]},"data":[{"url":"https://example.com/img.png"}]}

data: [DONE]`

	count = countOpenAIImageOutputsFromSSEBody(sseWithImageData)
	if count != 1 {
		t.Fatalf("expected 1 image for data array with image URL, got %d", count)
	}
}

func TestOpenAIImageOutputCounterCompletedItemRequiresImagePayload(t *testing.T) {
	jsonBody := []byte(`{
		"id": "resp_1",
		"object": "response",
		"status": "completed",
		"output": [
			{"id": "img_1", "type": "image_generation.completed", "status": "completed"}
		],
		"usage": {"input_tokens": 10, "output_tokens": 5}
	}`)

	count := countOpenAIResponseImageOutputsFromJSONBytes(jsonBody)
	if count != 0 {
		t.Fatalf("expected 0 images for completed image item without payload, got %d", count)
	}
}
