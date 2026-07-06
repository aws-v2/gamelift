package storage

import (
	"context"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.uber.org/zap"
)

type MinIOAdapter struct {
	client *minio.Client
	logger *zap.SugaredLogger
}

func NewMinIOAdapter(endpoint, accessKey, secretKey string, useSSL bool, logger *zap.SugaredLogger) (*MinIOAdapter, error) {
	logger.Infow("MINIO_CONNECT", "endpoint", endpoint, "use_ssl", useSSL)

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		logger.Errorw("MINIO_CONNECT_FAILED", "endpoint", endpoint, "error", err)
		return nil, fmt.Errorf("failed to create MinIO client: %w", err)
	}

	logger.Infow("MINIO_CONNECT_SUCCESS", "endpoint", endpoint, "use_ssl", useSSL)
	return &MinIOAdapter{client: client, logger: logger}, nil
}

// DownloadFile downloads an object from a bucket to a local file path,
// streaming directly to disk rather than loading into memory.
func (m *MinIOAdapter) DownloadFile(ctx context.Context, bucket, key, destPath string) error {
	m.logger.Infow("MINIO_DOWNLOAD_FILE", "bucket", bucket, "key", key, "dest", destPath)

	if err := m.client.FGetObject(ctx, bucket, key, destPath, minio.GetObjectOptions{}); err != nil {
		m.logger.Errorw("MINIO_DOWNLOAD_FILE_FAILED", "bucket", bucket, "key", key, "dest", destPath, "error", err)
		return fmt.Errorf("failed to download file from minio: %w", err)
	}

	m.logger.Infow("MINIO_DOWNLOAD_FILE_SUCCESS", "bucket", bucket, "key", key, "dest", destPath)
	return nil
}

// GetObjectStream returns an io.ReadCloser for the object.
// The caller is responsible for closing the stream.
func (m *MinIOAdapter) GetObjectStream(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	m.logger.Infow("MINIO_GET_OBJECT_STREAM", "bucket", bucket, "key", key)

	object, err := m.client.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		m.logger.Errorw("MINIO_GET_OBJECT_STREAM_FAILED", "bucket", bucket, "key", key, "error", err)
		return nil, fmt.Errorf("failed to get object stream: %w", err)
	}

	m.logger.Infow("MINIO_GET_OBJECT_STREAM_SUCCESS", "bucket", bucket, "key", key)
	return object, nil
}

// GetObjectInfo returns metadata about an object such as size and content type.
func (m *MinIOAdapter) GetObjectInfo(ctx context.Context, bucket, key string) (minio.ObjectInfo, error) {
	m.logger.Infow("MINIO_STAT_OBJECT", "bucket", bucket, "key", key)

	info, err := m.client.StatObject(ctx, bucket, key, minio.StatObjectOptions{})
	if err != nil {
		m.logger.Errorw("MINIO_STAT_OBJECT_FAILED", "bucket", bucket, "key", key, "error", err)
		return minio.ObjectInfo{}, err
	}

	m.logger.Infow("MINIO_STAT_OBJECT_SUCCESS",
		"bucket", bucket,
		"key", key,
		"size_bytes", info.Size,
		"content_type", info.ContentType,
		"last_modified", info.LastModified,
	)
	return info, nil
}