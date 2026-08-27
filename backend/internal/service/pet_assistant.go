package service

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"net/http"
	"path"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"golang.org/x/image/webp"
	"golang.org/x/sync/singleflight"
)

const (
	PetZIPMaxBytes          = 10 << 20
	PetZIPMaxEntries        = 8
	PetZIPMaxExpandedBytes  = 12 << 20
	PetSpritesheetMaxBytes  = 10 << 20
	PetQuestionMaxRunes     = 2000
	PetKnowledgeMaxDocs     = 500
	PetKnowledgeExcerptSize = 900
	// DefaultPetAssetID is the stable UUID of the catalog's default pet.
	DefaultPetAssetID   = "baf63c4a-f555-5eaf-acb7-47eaabdd1382"
	petCatalogURLPrefix = "https://raw.githubusercontent.com/legeling/awesome-codex-pet/main/pets/"
)

var (
	ErrPetAssetNotFound       = errors.New("pet asset not found")
	ErrPetConversationMissing = errors.New("pet conversation not found")
	ErrPetInvalidArchive      = errors.New("invalid Codex Pet archive")
	ErrPetKnowledgeNotFound   = errors.New("pet knowledge document not found")
)

type PetManifest struct {
	ID                  string `json:"id"`
	DisplayName         string `json:"displayName"`
	Description         string `json:"description"`
	SpriteVersionNumber int    `json:"spriteVersionNumber"`
	SpritesheetPath     string `json:"spritesheetPath"`
}

type PetAsset struct {
	ID            string    `json:"id"`
	OwnerUserID   *int64    `json:"-"`
	PetKey        string    `json:"pet_key"`
	DisplayName   string    `json:"display_name"`
	Description   string    `json:"description"`
	SpriteVersion int       `json:"sprite_version"`
	StorageKey    string    `json:"-"`
	SHA256        string    `json:"sha256"`
	SizeBytes     int64     `json:"size_bytes"`
	Width         int       `json:"width"`
	Height        int       `json:"height"`
	License       string    `json:"license"`
	CreatedAt     time.Time `json:"created_at"`
	AssetURL      string    `json:"asset_url"`
	IsBuiltIn     bool      `json:"is_builtin"`
}

type PetPreferences struct {
	UserID            int64    `json:"-"`
	SelectedAssetID   *string  `json:"selected_asset_id"`
	AssistantGroupID  *int64   `json:"assistant_group_id"`
	AssistantModel    string   `json:"assistant_model"`
	Enabled           bool     `json:"enabled"`
	Size              string   `json:"size"`
	Anchor            string   `json:"anchor"`
	ReducedMotion     bool     `json:"reduced_motion"`
	ActivityReactions bool     `json:"activity_reactions"`
	PositionX         *float64 `json:"position_x"`
	PositionY         *float64 `json:"position_y"`
}

type PetKnowledgeDocument struct {
	ID        int64     `json:"id"`
	Slug      string    `json:"slug"`
	Title     string    `json:"title"`
	ContentMD string    `json:"content_md"`
	Status    string    `json:"status"`
	Version   int       `json:"version"`
	UpdatedBy *int64    `json:"updated_by,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type PetCitation struct {
	DocumentID int64  `json:"document_id"`
	Slug       string `json:"slug"`
	Title      string `json:"title"`
	Version    int    `json:"version"`
	Excerpt    string `json:"excerpt"`
}

type PetMessage struct {
	ID             int64         `json:"id"`
	ConversationID string        `json:"conversation_id"`
	Role           string        `json:"role"`
	Content        string        `json:"content"`
	Citations      []PetCitation `json:"citations"`
	Refused        bool          `json:"refused"`
	CreatedAt      time.Time     `json:"created_at"`
}

type PetConversation struct {
	ID        string       `json:"id"`
	UserID    int64        `json:"-"`
	Title     string       `json:"title"`
	Messages  []PetMessage `json:"messages,omitempty"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

type PetRepository interface {
	ListAssets(ctx context.Context, userID int64) ([]PetAsset, error)
	CreateAsset(ctx context.Context, asset *PetAsset) error
	GetAsset(ctx context.Context, userID int64, assetID string) (*PetAsset, error)
	DeleteAsset(ctx context.Context, userID int64, assetID string) (*PetAsset, error)
	GetPreferences(ctx context.Context, userID int64) (*PetPreferences, error)
	UpsertPreferences(ctx context.Context, prefs PetPreferences) (*PetPreferences, error)
	ListPublishedKnowledge(ctx context.Context, limit int) ([]PetKnowledgeDocument, error)
	ListKnowledge(ctx context.Context) ([]PetKnowledgeDocument, error)
	CreateKnowledge(ctx context.Context, doc *PetKnowledgeDocument) error
	UpdateKnowledge(ctx context.Context, doc *PetKnowledgeDocument) error
	DeleteKnowledge(ctx context.Context, id int64) error
	CreateConversation(ctx context.Context, conversation *PetConversation) error
	GetConversation(ctx context.Context, userID int64, id string) (*PetConversation, error)
	AppendMessage(ctx context.Context, message *PetMessage) error
}

type PetAssetStorage interface {
	Put(ctx context.Context, key string, data []byte) error
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
}

type PreparedPetAsset struct {
	Manifest    PetManifest
	Spritesheet []byte
	SHA256      string
	Width       int
	Height      int
}

// PetAnswerInput carries only the authenticated user and selected group IDs
// needed to enter the normal billed gateway, plus published knowledge.
type PetAnswerInput struct {
	UserID           int64
	AssistantGroupID int64
	Question         string
	Citations        []PetCitation
}

type PetAnswerOutput struct {
	Content string
	Refused bool
}

type PetAnswerProvider interface {
	Answer(ctx context.Context, input PetAnswerInput) (PetAnswerOutput, error)
}

// GroundedPetAnswerProvider is the deterministic first-party implementation.
// A model-backed provider can replace it later without changing retrieval,
// permissions, conversations, or the public API contract.
type GroundedPetAnswerProvider struct{}

func NewGroundedPetAnswerProvider() *GroundedPetAnswerProvider {
	return &GroundedPetAnswerProvider{}
}

func (p *GroundedPetAnswerProvider) Answer(_ context.Context, input PetAnswerInput) (PetAnswerOutput, error) {
	answer, refused := composeGroundedPetAnswer(input.Citations)
	return PetAnswerOutput{Content: answer, Refused: refused}, nil
}

type PetAssistantService struct {
	repo           PetRepository
	storage        PetAssetStorage
	answerProvider PetAnswerProvider
	remoteClient   *http.Client
	catalogFetch   singleflight.Group
}

func NewPetAssistantService(repo PetRepository, storage PetAssetStorage, answerProvider PetAnswerProvider) *PetAssistantService {
	return &PetAssistantService{
		repo: repo, storage: storage, answerProvider: answerProvider,
		remoteClient: &http.Client{Timeout: 45 * time.Second},
	}
}

func ProvidePetAnswerProvider(provider *GroundedPetAnswerProvider) PetAnswerProvider {
	return provider
}

func (s *PetAssistantService) ListAssets(ctx context.Context, userID int64) ([]PetAsset, error) {
	items, err := s.repo.ListAssets(ctx, userID)
	for i := range items {
		items[i].AssetURL = "/api/v1/pet/assets/" + items[i].ID + "/spritesheet"
	}
	return items, err
}

func (s *PetAssistantService) ImportAsset(ctx context.Context, userID int64, archive io.Reader) (*PetAsset, error) {
	prepared, err := PreparePetArchive(archive)
	if err != nil {
		return nil, err
	}
	id := uuid.NewString()
	key := fmt.Sprintf("users/%d/%s/spritesheet.webp", userID, id)
	if err := s.storage.Put(ctx, key, prepared.Spritesheet); err != nil {
		return nil, fmt.Errorf("store pet spritesheet: %w", err)
	}
	asset := &PetAsset{
		ID: id, OwnerUserID: &userID, PetKey: prepared.Manifest.ID,
		DisplayName: prepared.Manifest.DisplayName, Description: prepared.Manifest.Description,
		SpriteVersion: prepared.Manifest.SpriteVersionNumber, StorageKey: key, SHA256: prepared.SHA256,
		SizeBytes: int64(len(prepared.Spritesheet)), Width: prepared.Width, Height: prepared.Height,
	}
	if err := s.repo.CreateAsset(ctx, asset); err != nil {
		_ = s.storage.Delete(ctx, key)
		return nil, err
	}
	asset.AssetURL = "/api/v1/pet/assets/" + asset.ID + "/spritesheet"
	return asset, nil
}

func (s *PetAssistantService) OpenAsset(ctx context.Context, userID int64, assetID string) (io.ReadCloser, *PetAsset, error) {
	asset, err := s.repo.GetAsset(ctx, userID, assetID)
	if err != nil {
		return nil, nil, err
	}
	if strings.HasPrefix(asset.StorageKey, petCatalogURLPrefix) {
		reader, openErr := s.openCatalogAsset(ctx, asset)
		return reader, asset, openErr
	}
	r, err := s.storage.Open(ctx, asset.StorageKey)
	return r, asset, err
}

// openCatalogAsset lazily mirrors a built-in spritesheet into the persistent
// pet asset directory. The database keeps the upstream URL as provenance, but
// every request after the first one is served from local storage.
func (s *PetAssistantService) openCatalogAsset(ctx context.Context, asset *PetAsset) (io.ReadCloser, error) {
	if s == nil || s.storage == nil || asset == nil {
		return nil, fmt.Errorf("pet asset storage is unavailable")
	}
	cacheKey, err := petCatalogCacheKey(asset)
	if err != nil {
		return nil, err
	}
	if reader, openErr := s.storage.Open(ctx, cacheKey); openErr == nil {
		return reader, nil
	}

	_, err, _ = s.catalogFetch.Do(cacheKey, func() (any, error) {
		// Another request may have completed the download between the first Open
		// and joining this singleflight call.
		if reader, openErr := s.storage.Open(ctx, cacheKey); openErr == nil {
			_ = reader.Close()
			return struct{}{}, nil
		}
		data, downloadErr := s.downloadCatalogAsset(ctx, asset)
		if downloadErr != nil {
			return nil, downloadErr
		}
		if putErr := s.storage.Put(ctx, cacheKey, data); putErr != nil {
			return nil, fmt.Errorf("cache catalog pet: %w", putErr)
		}
		return struct{}{}, nil
	})
	if err != nil {
		return nil, err
	}
	reader, err := s.storage.Open(ctx, cacheKey)
	if err != nil {
		return nil, fmt.Errorf("open cached catalog pet: %w", err)
	}
	return reader, nil
}

func (s *PetAssistantService) downloadCatalogAsset(ctx context.Context, asset *PetAsset) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.StorageKey, nil)
	if err != nil {
		return nil, err
	}
	client := s.remoteClient
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("catalog pet returned HTTP %d", response.StatusCode)
	}
	if response.ContentLength > PetSpritesheetMaxBytes {
		return nil, fmt.Errorf("catalog pet spritesheet is too large")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, PetSpritesheetMaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("download catalog pet: %w", err)
	}
	if len(data) == 0 || len(data) > PetSpritesheetMaxBytes {
		return nil, fmt.Errorf("catalog pet spritesheet is too large")
	}
	if asset.SizeBytes > 0 && int64(len(data)) != asset.SizeBytes {
		return nil, fmt.Errorf("catalog pet size mismatch: expected %d bytes, got %d", asset.SizeBytes, len(data))
	}
	sum := sha256.Sum256(data)
	if !strings.EqualFold(hex.EncodeToString(sum[:]), strings.TrimSpace(asset.SHA256)) {
		return nil, fmt.Errorf("catalog pet checksum mismatch")
	}
	if len(data) < 12 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
		return nil, fmt.Errorf("catalog pet is not WebP")
	}
	return data, nil
}

func petCatalogCacheKey(asset *PetAsset) (string, error) {
	if asset == nil {
		return "", fmt.Errorf("pet asset is missing")
	}
	checksum := strings.ToLower(strings.TrimSpace(asset.SHA256))
	decoded, err := hex.DecodeString(checksum)
	if err != nil || len(decoded) != sha256.Size {
		return "", fmt.Errorf("catalog pet checksum is invalid")
	}
	return path.Join("catalog", checksum+".webp"), nil
}

func (s *PetAssistantService) DeleteAsset(ctx context.Context, userID int64, assetID string) error {
	asset, err := s.repo.DeleteAsset(ctx, userID, assetID)
	if err != nil {
		return err
	}
	// The database is the source of truth for ownership and selection. Storage
	// cleanup is best-effort so a missing or temporarily unavailable file does
	// not make a successfully deleted asset appear to have failed.
	_ = s.storage.Delete(ctx, asset.StorageKey)
	return nil
}

func (s *PetAssistantService) GetPreferences(ctx context.Context, userID int64) (*PetPreferences, error) {
	return s.repo.GetPreferences(ctx, userID)
}

func (s *PetAssistantService) SavePreferences(ctx context.Context, prefs PetPreferences) (*PetPreferences, error) {
	prefs.AssistantModel = strings.TrimSpace(prefs.AssistantModel)
	if len([]rune(prefs.AssistantModel)) > 160 {
		return nil, fmt.Errorf("assistant model must not exceed 160 characters")
	}
	if prefs.Size != "small" && prefs.Size != "medium" && prefs.Size != "large" {
		return nil, fmt.Errorf("invalid pet size")
	}
	if prefs.Anchor != "bottom-left" && prefs.Anchor != "bottom-right" {
		return nil, fmt.Errorf("invalid pet anchor")
	}
	if (prefs.PositionX == nil) != (prefs.PositionY == nil) {
		return nil, fmt.Errorf("pet position coordinates must be provided together")
	}
	if prefs.PositionX != nil && (*prefs.PositionX < 0 || *prefs.PositionX > 1 || *prefs.PositionY < 0 || *prefs.PositionY > 1) {
		return nil, fmt.Errorf("pet position coordinates must be between 0 and 1")
	}
	if prefs.SelectedAssetID != nil {
		if _, err := s.repo.GetAsset(ctx, prefs.UserID, *prefs.SelectedAssetID); err != nil {
			return nil, err
		}
	}
	return s.repo.UpsertPreferences(ctx, prefs)
}

func (s *PetAssistantService) Ask(ctx context.Context, userID int64, conversationID, question string) (*PetConversation, *PetMessage, error) {
	question = strings.TrimSpace(question)
	if question == "" || len([]rune(question)) > PetQuestionMaxRunes {
		return nil, nil, fmt.Errorf("question must contain 1-%d characters", PetQuestionMaxRunes)
	}
	prefs, err := s.repo.GetPreferences(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	if prefs.AssistantGroupID == nil || *prefs.AssistantGroupID <= 0 {
		return nil, nil, fmt.Errorf("select an AI support group in Pet hall first")
	}
	var conversation *PetConversation
	if strings.TrimSpace(conversationID) == "" {
		title := truncateRunes(question, 80)
		conversation = &PetConversation{ID: uuid.NewString(), UserID: userID, Title: title}
		err = s.repo.CreateConversation(ctx, conversation)
	} else {
		conversation, err = s.repo.GetConversation(ctx, userID, conversationID)
	}
	if err != nil {
		return nil, nil, err
	}
	userMessage := &PetMessage{ConversationID: conversation.ID, Role: "user", Content: question, Citations: []PetCitation{}}
	if err := s.repo.AppendMessage(ctx, userMessage); err != nil {
		return nil, nil, err
	}
	docs, err := s.repo.ListPublishedKnowledge(ctx, PetKnowledgeMaxDocs)
	if err != nil {
		return nil, nil, err
	}
	citations := retrievePetKnowledge(question, docs, 3)
	answer, err := s.answerProvider.Answer(ctx, PetAnswerInput{
		UserID: userID, AssistantGroupID: *prefs.AssistantGroupID,
		Question: question, Citations: citations,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("answer pet question: %w", err)
	}
	assistant := &PetMessage{ConversationID: conversation.ID, Role: "assistant", Content: answer.Content, Citations: citations, Refused: answer.Refused}
	if err := s.repo.AppendMessage(ctx, assistant); err != nil {
		return nil, nil, err
	}
	return conversation, assistant, nil
}

func (s *PetAssistantService) GetConversation(ctx context.Context, userID int64, id string) (*PetConversation, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, ErrPetConversationMissing
	}
	return s.repo.GetConversation(ctx, userID, id)
}

func (s *PetAssistantService) ListKnowledge(ctx context.Context) ([]PetKnowledgeDocument, error) {
	return s.repo.ListKnowledge(ctx)
}

func (s *PetAssistantService) SaveKnowledge(ctx context.Context, actorID int64, id int64, doc PetKnowledgeDocument) (*PetKnowledgeDocument, error) {
	doc.Slug = strings.TrimSpace(doc.Slug)
	doc.Title = strings.TrimSpace(doc.Title)
	doc.ContentMD = strings.TrimSpace(doc.ContentMD)
	doc.Status = strings.TrimSpace(doc.Status)
	if doc.Slug == "" || len(doc.Slug) > 120 {
		return nil, fmt.Errorf("knowledge slug is required and must not exceed 120 characters")
	}
	for _, r := range doc.Slug {
		if !(r == '-' || r == '_' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9') {
			return nil, fmt.Errorf("knowledge slug may contain only lowercase letters, numbers, hyphens and underscores")
		}
	}
	if doc.Title == "" || len([]rune(doc.Title)) > 240 {
		return nil, fmt.Errorf("knowledge title is required and must not exceed 240 characters")
	}
	if doc.ContentMD == "" || len([]byte(doc.ContentMD)) > 256<<10 {
		return nil, fmt.Errorf("knowledge content is required and must not exceed 256 KiB")
	}
	if doc.Status != "draft" && doc.Status != "published" && doc.Status != "archived" {
		return nil, fmt.Errorf("knowledge status must be draft, published or archived")
	}
	doc.ID = id
	doc.UpdatedBy = &actorID
	if id == 0 {
		if err := s.repo.CreateKnowledge(ctx, &doc); err != nil {
			return nil, err
		}
	} else if err := s.repo.UpdateKnowledge(ctx, &doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

func (s *PetAssistantService) DeleteKnowledge(ctx context.Context, id int64) error {
	return s.repo.DeleteKnowledge(ctx, id)
}

func PreparePetArchive(r io.Reader) (*PreparedPetAsset, error) {
	data, err := io.ReadAll(io.LimitReader(r, PetZIPMaxBytes+1))
	if err != nil || len(data) == 0 || len(data) > PetZIPMaxBytes {
		return nil, fmt.Errorf("%w: archive exceeds %d bytes", ErrPetInvalidArchive, PetZIPMaxBytes)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil || len(zr.File) == 0 || len(zr.File) > PetZIPMaxEntries {
		return nil, fmt.Errorf("%w: invalid ZIP structure", ErrPetInvalidArchive)
	}
	files := make(map[string][]byte, 2)
	var expanded uint64
	for _, f := range zr.File {
		clean := path.Clean(strings.ReplaceAll(f.Name, "\\", "/"))
		if strings.HasPrefix(clean, "../") || strings.HasPrefix(clean, "/") || clean == ".." {
			return nil, fmt.Errorf("%w: unsafe archive path", ErrPetInvalidArchive)
		}
		if f.FileInfo().IsDir() {
			continue
		}
		base := path.Base(clean)
		if clean != base {
			return nil, fmt.Errorf("%w: files must be at the ZIP root", ErrPetInvalidArchive)
		}
		if base != "pet.json" && base != "spritesheet.webp" {
			return nil, fmt.Errorf("%w: unexpected file %s", ErrPetInvalidArchive, base)
		}
		expanded += f.UncompressedSize64
		if expanded > PetZIPMaxExpandedBytes || f.UncompressedSize64 > PetSpritesheetMaxBytes {
			return nil, fmt.Errorf("%w: expanded content too large", ErrPetInvalidArchive)
		}
		rc, openErr := f.Open()
		if openErr != nil {
			return nil, fmt.Errorf("%w: open %s", ErrPetInvalidArchive, base)
		}
		body, readErr := io.ReadAll(io.LimitReader(rc, PetSpritesheetMaxBytes+1))
		_ = rc.Close()
		if readErr != nil || len(body) > PetSpritesheetMaxBytes {
			return nil, fmt.Errorf("%w: read %s", ErrPetInvalidArchive, base)
		}
		if _, duplicate := files[base]; duplicate {
			return nil, fmt.Errorf("%w: duplicate %s", ErrPetInvalidArchive, base)
		}
		files[base] = body
	}
	manifestBytes, manifestOK := files["pet.json"]
	sprite, spriteOK := files["spritesheet.webp"]
	if !manifestOK || !spriteOK {
		return nil, fmt.Errorf("%w: pet.json and spritesheet.webp are required", ErrPetInvalidArchive)
	}
	var manifest PetManifest
	dec := json.NewDecoder(bytes.NewReader(manifestBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&manifest); err != nil {
		return nil, fmt.Errorf("%w: malformed pet.json: %v", ErrPetInvalidArchive, err)
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return nil, fmt.Errorf("%w: pet.json must contain exactly one object", ErrPetInvalidArchive)
	}
	manifest.ID = strings.TrimSpace(manifest.ID)
	manifest.DisplayName = strings.TrimSpace(manifest.DisplayName)
	manifest.Description = strings.TrimSpace(manifest.Description)
	if manifest.ID == "" || len(manifest.ID) > 96 || manifest.DisplayName == "" || len([]rune(manifest.DisplayName)) > 120 {
		return nil, fmt.Errorf("%w: invalid pet identity", ErrPetInvalidArchive)
	}
	if len([]rune(manifest.Description)) > 1000 {
		return nil, fmt.Errorf("%w: pet description is too long", ErrPetInvalidArchive)
	}
	if manifest.SpritesheetPath != "spritesheet.webp" {
		return nil, fmt.Errorf("%w: spritesheetPath must be spritesheet.webp", ErrPetInvalidArchive)
	}
	if manifest.SpriteVersionNumber != 1 && manifest.SpriteVersionNumber != 2 {
		return nil, fmt.Errorf("%w: unsupported sprite version", ErrPetInvalidArchive)
	}
	if len(sprite) < 12 || string(sprite[:4]) != "RIFF" || string(sprite[8:12]) != "WEBP" {
		return nil, fmt.Errorf("%w: spritesheet is not WebP", ErrPetInvalidArchive)
	}
	atlas, err := webp.Decode(bytes.NewReader(sprite))
	if err != nil {
		return nil, fmt.Errorf("%w: invalid WebP", ErrPetInvalidArchive)
	}
	expectedHeight := 1872
	if manifest.SpriteVersionNumber == 2 {
		expectedHeight = 2288
	}
	bounds := atlas.Bounds()
	if bounds.Dx() != 1536 || bounds.Dy() != expectedHeight {
		return nil, fmt.Errorf("%w: expected 1536x%d, got %dx%d", ErrPetInvalidArchive, expectedHeight, bounds.Dx(), bounds.Dy())
	}
	if err := validatePetAtlasImage(atlas, manifest.SpriteVersionNumber); err != nil {
		return nil, err
	}
	hash := sha256.Sum256(sprite)
	return &PreparedPetAsset{Manifest: manifest, Spritesheet: sprite, SHA256: hex.EncodeToString(hash[:]), Width: bounds.Dx(), Height: bounds.Dy()}, nil
}

func validatePetAtlasImage(atlas image.Image, version int) error {
	rows := 9
	if version == 2 {
		rows = 11
	}
	bounds := atlas.Bounds()
	if bounds.Dx() != 1536 || bounds.Dy() != rows*208 {
		return fmt.Errorf("%w: invalid atlas dimensions", ErrPetInvalidArchive)
	}
	usedColumns := []int{6, 8, 8, 4, 5, 8, 6, 6, 6, 8, 8}
	for row := 0; row < rows; row++ {
		for column := 0; column < 8; column++ {
			hasAlpha := petCellHasAlpha(atlas, bounds.Min.X+column*192, bounds.Min.Y+row*208)
			if column < usedColumns[row] && !hasAlpha {
				return fmt.Errorf("%w: row %d column %d is empty", ErrPetInvalidArchive, row, column)
			}
			if column >= usedColumns[row] && hasAlpha {
				return fmt.Errorf("%w: row %d column %d must be transparent", ErrPetInvalidArchive, row, column)
			}
		}
	}
	return nil
}

func petCellHasAlpha(atlas image.Image, startX, startY int) bool {
	for y := startY; y < startY+208; y++ {
		for x := startX; x < startX+192; x++ {
			_, _, _, alpha := atlas.At(x, y).RGBA()
			if alpha != 0 {
				return true
			}
		}
	}
	return false
}

func retrievePetKnowledge(question string, docs []PetKnowledgeDocument, limit int) []PetCitation {
	queryTokens := petSearchTokens(question)
	type scored struct {
		doc   PetKnowledgeDocument
		score int
	}
	scores := make([]scored, 0, len(docs))
	for _, doc := range docs {
		haystack := strings.ToLower(doc.Title + "\n" + doc.ContentMD)
		score := 0
		for _, token := range queryTokens {
			if strings.Contains(strings.ToLower(doc.Title), token) {
				score += 4
			}
			if strings.Contains(haystack, token) {
				score++
			}
		}
		if score > 0 {
			scores = append(scores, scored{doc: doc, score: score})
		}
	}
	sort.SliceStable(scores, func(i, j int) bool { return scores[i].score > scores[j].score })
	if len(scores) > limit {
		scores = scores[:limit]
	}
	result := make([]PetCitation, 0, len(scores))
	for _, item := range scores {
		result = append(result, PetCitation{DocumentID: item.doc.ID, Slug: item.doc.Slug, Title: item.doc.Title, Version: item.doc.Version, Excerpt: truncateRunes(strings.TrimSpace(item.doc.ContentMD), PetKnowledgeExcerptSize)})
	}
	return result
}

func composeGroundedPetAnswer(citations []PetCitation) (string, bool) {
	if len(citations) == 0 {
		return "这个问题在 IKIK 已发布的帮助内容中没有可靠依据。我只能回答 IKIK 账号、API、模型、计费、分组、支付和控制台使用相关问题；请换一种说法，或联系人工客服。", true
	}
	var b strings.Builder
	b.WriteString("我在 IKIK 的已发布帮助内容里找到了这些信息：\n\n")
	for i, citation := range citations {
		fmt.Fprintf(&b, "%d. %s\n%s\n\n", i+1, citation.Title, citation.Excerpt)
	}
	b.WriteString("以上回答只依据列出的 IKIK 文档；若你的实际页面或错误信息与文档不一致，请提供具体提示。")
	return strings.TrimSpace(b.String()), false
}

func petSearchTokens(value string) []string {
	parts := strings.FieldsFunc(strings.ToLower(value), func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r)
	})
	seen := map[string]struct{}{}
	result := make([]string, 0, len(parts)*4)
	stopTokens := map[string]struct{}{
		"什么": {}, "怎么": {}, "为什么": {}, "是否": {}, "可以": {}, "如何": {},
		"这个": {}, "那个": {}, "需要": {}, "请问": {}, "现在": {}, "用户": {},
	}
	appendToken := func(token string) {
		token = strings.TrimSpace(token)
		if len([]rune(token)) < 2 {
			return
		}
		if _, stop := stopTokens[token]; stop {
			return
		}
		if _, ok := seen[token]; ok {
			return
		}
		seen[token] = struct{}{}
		result = append(result, token)
	}
	for _, part := range parts {
		runes := []rune(strings.TrimSpace(part))
		if len(runes) < 2 {
			continue
		}
		appendToken(string(runes))
		if containsHan(runes) {
			for size := 4; size >= 2; size-- {
				for start := 0; start+size <= len(runes); start++ {
					appendToken(string(runes[start : start+size]))
				}
			}
		}
	}
	return result
}

func containsHan(value []rune) bool {
	for _, r := range value {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

func truncateRunes(value string, max int) string {
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max]) + "..."
}
