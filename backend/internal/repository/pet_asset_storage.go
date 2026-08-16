package repository

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"ikik-api/internal/config"
	"ikik-api/internal/service"
)

type localPetAssetStorage struct {
	root string
}

func NewPetAssetStorage(cfg *config.Config) service.PetAssetStorage {
	dataDir := strings.TrimSpace(cfg.Pet.DataDir)
	if dataDir == "" {
		dataDir = strings.TrimSpace(cfg.Pricing.DataDir)
	}
	if dataDir == "" {
		dataDir = "./data"
	}
	return &localPetAssetStorage{root: filepath.Join(dataDir, "pet-assets")}
}

func (s *localPetAssetStorage) resolve(key string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(key))
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid pet storage key")
	}
	full := filepath.Join(s.root, clean)
	rel, err := filepath.Rel(s.root, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid pet storage key")
	}
	return full, nil
}

func (s *localPetAssetStorage) Put(_ context.Context, key string, data []byte) error {
	full, err := s.resolve(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0750); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(full), ".pet-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if err := tmp.Chmod(0640); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, full)
}

func (s *localPetAssetStorage) Open(_ context.Context, key string) (io.ReadCloser, error) {
	full, err := s.resolve(key)
	if err != nil {
		return nil, err
	}
	return os.Open(full)
}

func (s *localPetAssetStorage) Delete(_ context.Context, key string) error {
	full, err := s.resolve(key)
	if err != nil {
		return err
	}
	err = os.Remove(full)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
