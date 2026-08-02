// Package storage puts and removes objects in an S3 bucket. Every object
// is written with SSE-S3 (AES-256) server-side encryption, so uploaded
// files are encrypted at rest without any client-side key management.
package storage

import (
	"bytes"
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// Client wraps the S3 operations a typical app needs: upload, delete, and
// building the public URL for a key.
type Client struct {
	s3     *s3.Client
	bucket string
	region string
}

// New builds a Client authenticated with a static access key/secret pair
// (not an IAM role — for apps that run outside AWS, e.g. on a VPS or
// locally).
func New(ctx context.Context, region, accessKeyID, secretAccessKey, bucket string) (*Client, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, ""),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("storage: loading aws config: %w", err)
	}

	return &Client{s3: s3.NewFromConfig(cfg), bucket: bucket, region: region}, nil
}

// Put uploads data under key with SSE-S3 encryption and returns its S3
// URL, setting a cache-forever Cache-Control header. This is only safe
// because it's the caller's responsibility to write each object under a
// fresh, never-reused key (e.g. a UUID) — Put never overwrites an existing
// key in place, so the object at a given URL never changes after it's
// written, and every client (browsers, CDNs, mobile image caches) can
// therefore treat it as immutable instead of re-fetching or revalidating
// on every load.
func (c *Client) Put(ctx context.Context, key, contentType string, data []byte) (string, error) {
	_, err := c.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:               aws.String(c.bucket),
		Key:                  aws.String(key),
		Body:                 bytes.NewReader(data),
		ContentType:          aws.String(contentType),
		ServerSideEncryption: types.ServerSideEncryptionAes256,
		CacheControl:         aws.String("public, max-age=31536000, immutable"),
	})
	if err != nil {
		return "", fmt.Errorf("storage: put %q: %w", key, err)
	}

	return c.URL(key), nil
}

// URL returns key's public S3 URL. Only meaningful for keys under a prefix
// the bucket policy actually allows public reads on.
func (c *Client) URL(key string) string {
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", c.bucket, c.region, key)
}

// Delete removes every given key in one batch request. A no-op for an
// empty slice, so callers don't need to special-case "nothing to delete".
func (c *Client) Delete(ctx context.Context, keys []string) error {
	if len(keys) == 0 {
		return nil
	}

	objects := make([]types.ObjectIdentifier, len(keys))
	for i, k := range keys {
		objects[i] = types.ObjectIdentifier{Key: aws.String(k)}
	}

	_, err := c.s3.DeleteObjects(ctx, &s3.DeleteObjectsInput{
		Bucket: aws.String(c.bucket),
		Delete: &types.Delete{Objects: objects},
	})
	if err != nil {
		return fmt.Errorf("storage: delete objects: %w", err)
	}

	return nil
}
