package repository

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"ikik-api/internal/service"
)

type shopFileCardOSSStore struct {
	client *s3.Client
	bucket string
}

func NewShopFileCardObjectStoreFactory() service.ShopFileCardObjectStoreFactory {
	return func(ctx context.Context, cfg service.ShopFileCardStorageConfig) (service.ShopFileCardObjectStore, error) {
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
		return &shopFileCardOSSStore{
			client: client,
			bucket: strings.TrimSpace(cfg.Bucket),
		}, nil
	}
}

func (s *shopFileCardOSSStore) Upload(ctx context.Context, key string, body io.Reader, contentType string) error {
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
		return mapObjectStorageError("upload shop file card object", err)
	}
	return nil
}

func (s *shopFileCardOSSStore) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &s.bucket,
		Key:    &key,
	})
	if err != nil {
		return nil, mapObjectStorageError("download shop file card object", err)
	}
	return result.Body, nil
}

func (s *shopFileCardOSSStore) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &s.bucket,
		Key:    &key,
	})
	if err != nil {
		return mapObjectStorageError("delete shop file card object", err)
	}
	return nil
}

func (s *shopFileCardOSSStore) HeadBucket(ctx context.Context) error {
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: &s.bucket})
	if err != nil {
		return mapObjectStorageError("test shop file card bucket", err)
	}
	return nil
}
