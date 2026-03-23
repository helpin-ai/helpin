package storage

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3Client wraps the AWS S3 presign client for generating presigned URLs and deleting objects.
type S3Client struct {
	client        *s3.Client
	presignClient *s3.PresignClient
	bucket        string
	endpointURL   string
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
		endpointURL:   endpointURL,
	}
}

// EnsureCORS sets a permissive CORS policy on the bucket so that browsers
// on any origin can upload files via presigned PUT URLs.
// Safe to call on every startup — it overwrites the existing CORS config.
func (s *S3Client) EnsureCORS(ctx context.Context) error {
	_, err := s.client.PutBucketCors(ctx, &s3.PutBucketCorsInput{
		Bucket: aws.String(s.bucket),
		CORSConfiguration: &s3types.CORSConfiguration{
			CORSRules: []s3types.CORSRule{
				{
					AllowedOrigins: []string{"*"},
					AllowedMethods: []string{"GET", "PUT", "HEAD"},
					AllowedHeaders: []string{"*"},
					ExposeHeaders:  []string{"ETag"},
					MaxAgeSeconds:  aws.Int32(3600),
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("put bucket CORS: %w", err)
	}
	return nil
}

// PublicURL constructs a direct public URL for the given storage key.
func (s *S3Client) PublicURL(key string) string {
	if s.endpointURL == "" {
		return ""
	}
	return fmt.Sprintf("%s/%s/%s", s.endpointURL, s.bucket, key)
}

// HasPublicURL returns true when the endpoint URL is configured (public access possible).
func (s *S3Client) HasPublicURL() bool {
	return s.endpointURL != ""
}

// GeneratePresignedPutURL generates a presigned PUT URL for uploading a file.
// When publicRead is true, the presigned URL includes x-amz-acl=public-read so
// the uploaded object is publicly accessible.
func (s *S3Client) GeneratePresignedPutURL(key, contentType string, size int64, publicRead bool) (string, error) {
	input := &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(size),
	}
	if publicRead {
		input.ACL = s3types.ObjectCannedACLPublicRead
	}

	result, err := s.presignClient.PresignPutObject(context.Background(), input, func(o *s3.PresignOptions) {
		o.Expires = 15 * time.Minute
	})
	if err != nil {
		return "", fmt.Errorf("generate presigned PUT URL: %w", err)
	}
	return result.URL, nil
}

// PutObject uploads an object directly to S3.
func (s *S3Client) PutObject(ctx context.Context, key, contentType string, size int64, body io.Reader, publicRead bool) error {
	input := &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(size),
		Body:          body,
	}
	if publicRead {
		input.ACL = s3types.ObjectCannedACLPublicRead
	}
	if _, err := s.client.PutObject(ctx, input); err != nil {
		return fmt.Errorf("put S3 object: %w", err)
	}
	return nil
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
