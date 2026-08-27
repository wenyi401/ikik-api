package service

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image"
	"image/color"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
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

func TestPetCatalogAssetDownloadsOnceThenUsesPersistentStorage(t *testing.T) {
	data := []byte("RIFF0000WEBP")
	sum := sha256.Sum256(data)
	var requests atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		_, _ = w.Write(data)
	}))
	defer upstream.Close()

	storage := newMemoryPetAssetStorage()
	asset := &PetAsset{
		StorageKey: upstream.URL, SHA256: hex.EncodeToString(sum[:]), SizeBytes: int64(len(data)),
	}
	firstService := NewPetAssistantService(nil, storage, nil)
	reader, err := firstService.openCatalogAsset(context.Background(), asset)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(reader)
	_ = reader.Close()
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("unexpected cached body %q, err=%v", got, err)
	}

	// A new service instance simulates a process restart. The local asset must
	// still be reusable without contacting GitHub again.
	secondService := NewPetAssistantService(nil, storage, nil)
	reader, err = secondService.openCatalogAsset(context.Background(), asset)
	if err != nil {
		t.Fatal(err)
	}
	_ = reader.Close()
	if got := requests.Load(); got != 1 {
		t.Fatalf("upstream requested %d times, want 1", got)
	}
}

func TestPetCatalogAssetSingleflightPreventsDuplicateDownloads(t *testing.T) {
	data := []byte("RIFF0000WEBP")
	sum := sha256.Sum256(data)
	var requests atomic.Int32
	release := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		<-release
		_, _ = w.Write(data)
	}))
	defer upstream.Close()

	storage := newMemoryPetAssetStorage()
	service := NewPetAssistantService(nil, storage, nil)
	asset := &PetAsset{
		StorageKey: upstream.URL, SHA256: hex.EncodeToString(sum[:]), SizeBytes: int64(len(data)),
	}
	const workers = 8
	var wg sync.WaitGroup
	wg.Add(workers)
	errs := make(chan error, workers)
	for range workers {
		go func() {
			defer wg.Done()
			reader, err := service.openCatalogAsset(context.Background(), asset)
			if err == nil {
				_ = reader.Close()
			}
			errs <- err
		}()
	}
	deadline := time.Now().Add(2 * time.Second)
	for requests.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	close(release)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("concurrent upstream requests = %d, want 1", got)
	}
}

func TestPetCatalogAssetRejectsChecksumMismatch(t *testing.T) {
	data := []byte("RIFF0000WEBP")
	badSum := sha256.Sum256([]byte("different"))
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(data)
	}))
	defer upstream.Close()
	storage := newMemoryPetAssetStorage()
	service := NewPetAssistantService(nil, storage, nil)
	asset := &PetAsset{
		StorageKey: upstream.URL, SHA256: hex.EncodeToString(badSum[:]), SizeBytes: int64(len(data)),
	}
	if _, err := service.openCatalogAsset(context.Background(), asset); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("expected checksum mismatch, got %v", err)
	}
	cacheKey, err := petCatalogCacheKey(asset)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := storage.Open(context.Background(), cacheKey); err == nil {
		t.Fatal("invalid upstream content must not be cached")
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

type memoryPetAssetStorage struct {
	mu    sync.Mutex
	files map[string][]byte
}

func newMemoryPetAssetStorage() *memoryPetAssetStorage {
	return &memoryPetAssetStorage{files: make(map[string][]byte)}
}

func (s *memoryPetAssetStorage) Put(_ context.Context, key string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.files[key] = append([]byte(nil), data...)
	return nil
}

func (s *memoryPetAssetStorage) Open(_ context.Context, key string) (io.ReadCloser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, ok := s.files[key]
	if !ok {
		return nil, errors.New("not found")
	}
	return io.NopCloser(bytes.NewReader(append([]byte(nil), data...))), nil
}

func (s *memoryPetAssetStorage) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.files, key)
	return nil
}
