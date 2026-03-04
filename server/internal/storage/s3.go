package storage

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Client wraps the AWS S3 presign client for generating presigned URLs and deleting objects.
type S3Client struct {
	client       *s3.Client
	presignClient *s3.PresignClient
	bucket       string
}

// NewS3Client creates a new S3Client from the given configuration.
// If accessKeyID is empty, S3 features are disabled and nil is returned.
func NewS3Client(accessKeyID, secretAccessKey, bucket, region, endpointURL string) *S3Client {
	if accessKeyID == "" || bucket == "" {
		return nil
	}

	cfg := aws.Config{
		Region:      region,
		Credentials: credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, ""),
	}

	var opts []func(*s3.Options)
	if endpointURL != "" {
		opts = append(opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(endpointURL)
			o.UsePathStyle = true // Required for MinIO
		})
	}

	client := s3.NewFromConfig(cfg, opts...)
	presignClient := s3.NewPresignClient(client)

	return &S3Client{
		client:        client,
		presignClient: presignClient,
		bucket:        bucket,
	}
}

// GeneratePresignedPutURL generates a presigned PUT URL for uploading a file.
func (s *S3Client) GeneratePresignedPutURL(key, contentType string, size int64) (string, error) {
	input := &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(size),
	}

	result, err := s.presignClient.PresignPutObject(context.Background(), input, func(o *s3.PresignOptions) {
		o.Expires = 15 * time.Minute
	})
	if err != nil {
		return "", fmt.Errorf("generate presigned PUT URL: %w", err)
	}
	return result.URL, nil
}

// GeneratePresignedGetURL generates a presigned GET URL for downloading a file.
func (s *S3Client) GeneratePresignedGetURL(key, filename string) (string, error) {
	input := &s3.GetObjectInput{
		Bucket:                     aws.String(s.bucket),
		Key:                        aws.String(key),
		ResponseContentDisposition: aws.String(fmt.Sprintf(`attachment; filename="%s"`, url.PathEscape(filename))),
	}

	result, err := s.presignClient.PresignGetObject(context.Background(), input, func(o *s3.PresignOptions) {
		o.Expires = 1 * time.Hour
	})
	if err != nil {
		return "", fmt.Errorf("generate presigned GET URL: %w", err)
	}
	return result.URL, nil
}

// DeleteObject deletes an object from S3.
func (s *S3Client) DeleteObject(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("delete S3 object: %w", err)
	}
	return nil
}
