package repository

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"ikik-api/internal/config"
	"ikik-api/internal/service"
)

type receiptCodeOSSStore struct {
	client        *s3.Client
	bucket        string
	publicBaseURL string
}

func NewReceiptCodeObjectStoreFactory() service.ReceiptCodeObjectStoreFactory {
	return func(ctx context.Context, cfg config.ReceiptCodeStorageConfig) (service.ReceiptCodeObjectStore, error) {
		region := strings.TrimSpace(cfg.Region)
		if region == "" {
			region = "oss-cn-hangzhou"
		}

		awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
			awsconfig.WithRegion(region),
			awsconfig.WithCredentialsProvider(
				credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
			),
		)
		if err != nil {
			return nil, fmt.Errorf("load aws config: %w", err)
		}

		client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
			if cfg.Endpoint != "" {
				endpoint := strings.TrimRight(strings.TrimSpace(cfg.Endpoint), "/")
				o.BaseEndpoint = &endpoint
			}
			o.UsePathStyle = cfg.ForcePathStyle
			o.APIOptions = append(o.APIOptions, v4.SwapComputePayloadSHA256ForUnsignedPayloadMiddleware)
			o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		})

		return &receiptCodeOSSStore{
			client:        client,
			bucket:        strings.TrimSpace(cfg.Bucket),
			publicBaseURL: strings.TrimRight(strings.TrimSpace(cfg.PublicBaseURL), "/"),
		}, nil
	}
}

func (s *receiptCodeOSSStore) Upload(ctx context.Context, key string, body io.Reader, contentType string) error {
	data, err := io.ReadAll(body)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}
	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      &s.bucket,
		Key:         &key,
		Body:        bytes.NewReader(data),
		ContentType: &contentType,
	})
	if err != nil {
		return mapObjectStorageError("upload receipt code object", err)
	}
	return nil
}

func (s *receiptCodeOSSStore) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &s.bucket,
		Key:    &key,
	})
	if err != nil {
		return mapObjectStorageError("delete receipt code object", err)
	}
	return nil
}

func (s *receiptCodeOSSStore) PresignURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	presignClient := s3.NewPresignClient(s.client)
	result, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: &s.bucket,
		Key:    &key,
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", mapObjectStorageError("presign receipt code object", err)
	}
	return result.URL, nil
}

func (s *receiptCodeOSSStore) PublicURL(key string) string {
	if s.publicBaseURL == "" {
		return ""
	}
	return s.publicBaseURL + "/" + strings.TrimLeft(key, "/")
}
