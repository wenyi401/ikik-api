package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"ikik-api/internal/config"
)

func TestReceiptCodeUploadFallsBackToInlineStorageWhenOSSNotConfigured(t *testing.T) {
	ctx := context.Background()
	repo := &memoryReceiptCodeRepo{}
	svc := NewReceiptCodeService(repo, staticReceiptCodeConfigProvider{}, nil)
	image := tinyPNG(t)

	code, err := svc.Upload(ctx, ReceiptCodeUploadInput{
		UserID:        42,
		PaymentMethod: ReceiptCodePaymentMethodAlipay,
		FileName:      "qr.png",
		ContentType:   "image/png",
		Body:          bytes.NewReader(image),
		Size:          int64(len(image)),
	})
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	if code.StorageProvider != receiptCodeStorageProviderDB {
		t.Fatalf("StorageProvider = %q, want %q", code.StorageProvider, receiptCodeStorageProviderDB)
	}
	if !strings.HasPrefix(code.StorageKey, "receipt-codes-inline/42/alipay-") {
		t.Fatalf("StorageKey = %q", code.StorageKey)
	}
	if !strings.HasPrefix(code.URL, "data:image/png;base64,") {
		t.Fatalf("URL = %q, want inline png data URL", code.URL)
	}

	loaded, err := svc.Get(ctx, 42, ReceiptCodePaymentMethodAlipay)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if loaded == nil || loaded.URL != code.URL {
		t.Fatalf("Get() URL = %q, want %q", loadedURL(loaded), code.URL)
	}

	if err := svc.Delete(ctx, 42, ReceiptCodePaymentMethodAlipay); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
}

type staticReceiptCodeConfigProvider struct {
	cfg config.ReceiptCodeStorageConfig
	err error
}

func (p staticReceiptCodeConfigProvider) GetReceiptCodeStorageConfig(context.Context) (config.ReceiptCodeStorageConfig, error) {
	return p.cfg, p.err
}

type memoryReceiptCodeRepo struct {
	code *ReceiptCode
}

func (r *memoryReceiptCodeRepo) GetReceiptCode(_ context.Context, userID int64, paymentMethod string) (*ReceiptCode, error) {
	if r.code == nil || r.code.UserID != userID || r.code.PaymentMethod != paymentMethod {
		return nil, nil
	}
	return cloneReceiptCode(r.code), nil
}

func (r *memoryReceiptCodeRepo) UpsertReceiptCode(_ context.Context, input ReceiptCodeUpsertInput) (*ReceiptCode, error) {
	r.code = &ReceiptCode{
		ID:              1,
		UserID:          input.UserID,
		PaymentMethod:   input.PaymentMethod,
		StorageProvider: input.StorageProvider,
		StorageKey:      input.StorageKey,
		URL:             input.URL,
		ContentType:     input.ContentType,
		ByteSize:        input.ByteSize,
		SHA256:          input.SHA256,
	}
	return cloneReceiptCode(r.code), nil
}

func (r *memoryReceiptCodeRepo) DeleteReceiptCode(_ context.Context, userID int64, paymentMethod string) (*ReceiptCode, error) {
	if r.code == nil || r.code.UserID != userID || r.code.PaymentMethod != paymentMethod {
		return nil, nil
	}
	deleted := cloneReceiptCode(r.code)
	r.code = nil
	return deleted, nil
}

func (r *memoryReceiptCodeRepo) ReceiptCodeInUse(context.Context, string) (bool, error) {
	return false, nil
}

func cloneReceiptCode(code *ReceiptCode) *ReceiptCode {
	if code == nil {
		return nil
	}
	clone := *code
	return &clone
}

func loadedURL(code *ReceiptCode) string {
	if code == nil {
		return ""
	}
	return code.URL
}

func tinyPNG(t *testing.T) []byte {
	t.Helper()
	data, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/p9sAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatalf("decode tiny png: %v", err)
	}
	return data
}
