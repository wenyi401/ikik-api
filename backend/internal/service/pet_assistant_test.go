package service

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"io"
	"strings"
	"testing"
)

func TestPetSearchTokensChineseNGrams(t *testing.T) {
	tokens := petSearchTokens("为什么我的 API 账号一直显示限流？")
	want := []string{"api", "账号", "限流"}
	for _, token := range want {
		if !petContainsString(tokens, token) {
			t.Fatalf("expected token %q in %#v", token, tokens)
		}
	}
	if petContainsString(tokens, "为什么") {
		t.Fatalf("generic question word should not affect retrieval: %#v", tokens)
	}
}

func TestRetrievePetKnowledgeRanksRelevantChineseDocument(t *testing.T) {
	docs := []PetKnowledgeDocument{
		{ID: 1, Slug: "payments", Title: "支付配置", ContentMD: "支付宝和微信支付的回调地址配置。", Version: 2},
		{ID: 2, Slug: "rate-limit", Title: "账号限流状态", ContentMD: "OpenAI 额度重置后，可以在账号管理中手动刷新限流状态。", Version: 3},
	}
	citations := retrievePetKnowledge("为什么额度重置了账号还是显示限流", docs, 3)
	if len(citations) == 0 || citations[0].DocumentID != 2 {
		t.Fatalf("expected rate-limit document first, got %#v", citations)
	}
}

func TestGroundedPetAnswerProviderRefusesWithoutCitation(t *testing.T) {
	provider := NewGroundedPetAnswerProvider()
	answer, err := provider.Answer(context.Background(), PetAnswerInput{Question: "今天天气如何"})
	if err != nil {
		t.Fatal(err)
	}
	if !answer.Refused || !strings.Contains(answer.Content, "没有可靠依据") {
		t.Fatalf("expected grounded refusal, got %#v", answer)
	}
}

func TestPreparePetArchiveRejectsTraversal(t *testing.T) {
	archive := makePetTestZIP(t, map[string][]byte{
		"../pet.json":      []byte("{\"id\":\"bad\"}"),
		"spritesheet.webp": []byte("not-a-webp"),
	})
	_, err := PreparePetArchive(bytes.NewReader(archive))
	if !errors.Is(err, ErrPetInvalidArchive) || !strings.Contains(err.Error(), "unsafe archive path") {
		t.Fatalf("expected unsafe path error, got %v", err)
	}
}

func TestPreparePetArchiveRejectsUnexpectedFile(t *testing.T) {
	archive := makePetTestZIP(t, map[string][]byte{
		"pet.json":         []byte("{\"id\":\"bad\"}"),
		"spritesheet.webp": []byte("not-a-webp"),
		"notes.txt":        []byte("unexpected"),
	})
	_, err := PreparePetArchive(bytes.NewReader(archive))
	if !errors.Is(err, ErrPetInvalidArchive) || !strings.Contains(err.Error(), "unexpected file") {
		t.Fatalf("expected unexpected file error, got %v", err)
	}
}

func TestPreparePetArchiveRejectsNestedFiles(t *testing.T) {
	archive := makePetTestZIP(t, map[string][]byte{
		"pet/pet.json":     []byte("{\"id\":\"bad\"}"),
		"spritesheet.webp": []byte("not-a-webp"),
	})
	_, err := PreparePetArchive(bytes.NewReader(archive))
	if !errors.Is(err, ErrPetInvalidArchive) || !strings.Contains(err.Error(), "ZIP root") {
		t.Fatalf("expected ZIP root error, got %v", err)
	}
}

func TestValidatePetAtlasImageUsesStandardFrameCounts(t *testing.T) {
	atlas := image.NewNRGBA(image.Rect(0, 0, 1536, 1872))
	usedColumns := []int{6, 8, 8, 4, 5, 8, 6, 6, 6}
	for row, columns := range usedColumns {
		for column := 0; column < columns; column++ {
			atlas.SetNRGBA(column*192, row*208, color.NRGBA{R: 1, A: 255})
		}
	}
	if err := validatePetAtlasImage(atlas, 1); err != nil {
		t.Fatalf("expected valid standard atlas layout, got %v", err)
	}
	atlas.SetNRGBA(7*192, 0, color.NRGBA{R: 1, A: 255})
	if err := validatePetAtlasImage(atlas, 1); err == nil || !strings.Contains(err.Error(), "must be transparent") {
		t.Fatalf("expected unused cell rejection, got %v", err)
	}
}

func makePetTestZIP(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var body bytes.Buffer
	zw := zip.NewWriter(&body)
	for name, content := range files {
		writer, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(writer, bytes.NewReader(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return body.Bytes()
}

func petContainsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
