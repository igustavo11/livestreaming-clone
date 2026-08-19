package storage

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// R2Config holds Cloudflare R2 (S3-compatible) settings.
type R2Config struct {
	AccountID       string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
	PublicBaseURL   string
}

// Enabled reports whether enough config is present to talk to R2.
func (c R2Config) Enabled() bool {
	return c.AccountID != "" &&
		c.AccessKeyID != "" &&
		c.SecretAccessKey != "" &&
		c.Bucket != "" &&
		c.PublicBaseURL != ""
}

// R2 is an ObjectStorage backed by Cloudflare R2.
type R2 struct {
	client        *s3.Client
	bucket        string
	publicBaseURL string
}

// NewR2 builds an R2 client. Returns nil, nil when config is incomplete.
func NewR2(cfg R2Config) (*R2, error) {
	if !cfg.Enabled() {
		return nil, nil
	}

	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", cfg.AccountID)
	client := s3.New(s3.Options{
		BaseEndpoint: aws.String(endpoint),
		Region:       "auto",
		Credentials:  credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
	})

	return &R2{
		client:        client,
		bucket:        cfg.Bucket,
		publicBaseURL: strings.TrimRight(cfg.PublicBaseURL, "/"),
	}, nil
}

func (r *R2) Put(ctx context.Context, key, contentType string, body io.Reader, size int64) (string, error) {
	_, err := r.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(r.bucket),
		Key:           aws.String(key),
		Body:          body,
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(size),
	})
	if err != nil {
		return "", fmt.Errorf("r2 put: %w", err)
	}
	return r.publicBaseURL + "/" + key, nil
}

func (r *R2) Delete(ctx context.Context, key string) error {
	_, err := r.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("r2 delete: %w", err)
	}
	return nil
}

// KeyFromURL extracts the object key from a public URL under PublicBaseURL.
func KeyFromURL(publicBaseURL, publicURL string) string {
	base := strings.TrimRight(publicBaseURL, "/") + "/"
	if strings.HasPrefix(publicURL, base) {
		return strings.TrimPrefix(publicURL, base)
	}
	u, err := url.Parse(publicURL)
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(u.Path, "/")
}
