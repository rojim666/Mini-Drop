package apiserver

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type artifactStorage struct {
	backend         string
	artifactDir     string
	minioClient     *minio.Client
	minioSigner     *minio.Client
	minioBucket     string
	minioPresignTTL time.Duration
	uploaded        sync.Map
}

func newArtifactStorage(ctx context.Context, cfg Config) (*artifactStorage, error) {
	storage := &artifactStorage{
		backend:     strings.ToLower(strings.TrimSpace(cfg.StorageBackend)),
		artifactDir: cfg.ArtifactDir,
	}

	if storage.backend == "" || storage.backend == "local" {
		storage.backend = "local"
		return storage, nil
	}

	if storage.backend != "minio" {
		return nil, fmt.Errorf("unsupported MINIDROP_STORAGE_BACKEND %q; use local or minio", cfg.StorageBackend)
	}

	client, err := minio.New(cfg.MinIOEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.MinIOAccessKey, cfg.MinIOSecretKey, ""),
		Secure: cfg.MinIOUseSSL,
		Region: cfg.MinIORegion,
	})
	if err != nil {
		return nil, fmt.Errorf("create minio client: %w", err)
	}

	waitCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	for {
		select {
		case <-waitCtx.Done():
			return nil, fmt.Errorf("prepare minio bucket: %w", waitCtx.Err())
		default:
		}

		ok, err := client.BucketExists(waitCtx, cfg.MinIOBucket)
		if err == nil && ok {
			break
		}
		if err == nil && !ok {
			if err := client.MakeBucket(waitCtx, cfg.MinIOBucket, minio.MakeBucketOptions{}); err == nil {
				break
			}
		}
		time.Sleep(1 * time.Second)
	}

	storage.minioClient = client
	storage.minioSigner = client
	storage.minioBucket = cfg.MinIOBucket
	storage.minioPresignTTL = cfg.MinIOPresignTTL
	if raw := strings.TrimSpace(cfg.MinIOPublicEndpoint); raw != "" {
		base, err := normalizePublicEndpoint(raw, cfg.MinIOUseSSL)
		if err != nil {
			return nil, err
		}
		signer, err := minio.New(base.Host, &minio.Options{
			Creds:  credentials.NewStaticV4(cfg.MinIOAccessKey, cfg.MinIOSecretKey, ""),
			Secure: base.Scheme == "https",
			Region: cfg.MinIORegion,
		})
		if err != nil {
			return nil, fmt.Errorf("create minio signer: %w", err)
		}
		storage.minioSigner = signer
	}
	return storage, nil
}

func normalizePublicEndpoint(raw string, useSSL bool) (*url.URL, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, nil
	}
	if !strings.Contains(value, "://") {
		scheme := "http"
		if useSSL {
			scheme = "https"
		}
		value = scheme + "://" + value
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return nil, fmt.Errorf("parse minio public endpoint: %w", err)
	}
	if parsed.Scheme == "" {
		if useSSL {
			parsed.Scheme = "https"
		} else {
			parsed.Scheme = "http"
		}
	}
	return parsed, nil
}

func (s *artifactStorage) isMinIO() bool {
	return s != nil && s.backend == "minio" && s.minioClient != nil
}

func (s *artifactStorage) artifactURL(c *gin.Context, rel string) string {
	if strings.TrimSpace(rel) == "" {
		return ""
	}
	rel = cleanArtifactRelPath(rel)
	if !s.isMinIO() {
		return s.localArtifactURL(c, rel)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	objectName := rel
	absPath := filepath.Join(s.artifactDir, filepath.FromSlash(rel))
	if _, ok := s.uploaded.Load(objectName); !ok {
		if _, err := s.minioClient.FPutObject(ctx, s.minioBucket, objectName, absPath, minio.PutObjectOptions{}); err != nil {
			return s.localArtifactURL(c, rel)
		}
		s.uploaded.Store(objectName, struct{}{})
	}

	presigned, err := s.minioSigner.PresignedGetObject(ctx, s.minioBucket, objectName, s.minioPresignTTL, nil)
	if err != nil {
		return s.localArtifactURL(c, rel)
	}

	return presigned.String()
}

func (s *artifactStorage) localArtifactURL(c *gin.Context, rel string) string {
	rel = cleanArtifactRelPath(rel)
	scheme := "http"
	host := "127.0.0.1:8080"
	if c != nil && c.Request != nil {
		if c.Request.TLS != nil {
			scheme = "https"
		}
		if c.Request.Host != "" {
			host = c.Request.Host
		}
	}
	return fmt.Sprintf("%s://%s/artifacts/%s", scheme, host, path.Clean(rel))
}

func cleanArtifactRelPath(rel string) string {
	return strings.TrimPrefix(path.Clean("/"+filepath.ToSlash(strings.TrimSpace(rel))), "/")
}
