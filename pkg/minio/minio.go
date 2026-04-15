package minio

import (
	"context"
	"fmt"

	miniogo "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/zeelrupapara/seo-rank-guardian/config"
	"go.uber.org/zap"
)

// Client wraps the MinIO SDK client with bucket and logger context.
type Client struct {
	mc     *miniogo.Client
	bucket string
	log    *zap.SugaredLogger
}

// New creates a new MinIO client and verifies connectivity.
func New(cfg config.MinIOConfig, log *zap.SugaredLogger) (*Client, error) {
	mc, err := miniogo.New(cfg.Endpoint, &miniogo.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("minio: failed to create client: %w", err)
	}

	log.Infof("MinIO client initialised (endpoint=%s, bucket=%s)", cfg.Endpoint, cfg.Bucket)
	return &Client{mc: mc, bucket: cfg.Bucket, log: log}, nil
}

// EnsureBucket creates the bucket if it does not already exist.
func (c *Client) EnsureBucket(ctx context.Context) error {
	exists, err := c.mc.BucketExists(ctx, c.bucket)
	if err != nil {
		return fmt.Errorf("minio: checking bucket: %w", err)
	}
	if exists {
		return nil
	}
	if err := c.mc.MakeBucket(ctx, c.bucket, miniogo.MakeBucketOptions{}); err != nil {
		return fmt.Errorf("minio: creating bucket %q: %w", c.bucket, err)
	}
	c.log.Infof("MinIO: created bucket %q", c.bucket)
	return nil
}

// UploadFile uploads the file at filePath to the bucket using objectName as the key.
func (c *Client) UploadFile(ctx context.Context, objectName, filePath string) error {
	info, err := c.mc.FPutObject(ctx, c.bucket, objectName, filePath, miniogo.PutObjectOptions{
		ContentType: "application/octet-stream",
	})
	if err != nil {
		return fmt.Errorf("minio: upload %q → %q: %w", filePath, objectName, err)
	}
	c.log.Infof("MinIO: uploaded %q (%d bytes) to bucket %q", objectName, info.Size, c.bucket)
	return nil
}
