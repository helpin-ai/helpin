package storage

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"
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
	publicBaseURL string
	appBaseURL    string
	privateBucket bool
}

// NewS3Client creates a new S3Client from the given configuration.
// If accessKeyID is empty, S3 features are disabled and nil is returned.
func NewS3Client(accessKeyID, secretAccessKey, bucket, region, endpointURL, publicBaseURL string, presignEndpoint ...string) *S3Client {
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
	// Sign against the externally reachable S3 origin without changing the
	// internal transport used for deletion, server uploads, and bucket setup.
	if len(presignEndpoint) > 0 && presignEndpoint[0] != "" {
		browserClient := s3.NewFromConfig(cfg, func(o *s3.Options) { o.BaseEndpoint = aws.String(presignEndpoint[0]); o.UsePathStyle = true })
		presignClient = s3.NewPresignClient(browserClient)
	}

	return &S3Client{
		client:        client,
		presignClient: presignClient,
		bucket:        bucket,
		endpointURL:   endpointURL,
		publicBaseURL: publicBaseURL,
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
	if s.privateBucket && s.appBaseURL != "" && strings.HasPrefix(key, "docs-import/") {
		parts := strings.SplitN(key, "/", 3)
		if len(parts) == 3 {
			return s.appBaseURL + "/api/docs/images/content?workspace_id=" + url.QueryEscape(parts[1]) + "&key=" + url.QueryEscape(key)
		}
	}
	if s.privateBucket {
		if !PublicAssetKey(key) {
			return ""
		}
		return s.appBaseURL + "/api/public/assets/" + key
	}

	baseURL := s.publicBaseURL
	if baseURL == "" {
		baseURL = s.endpointURL
	}
	if baseURL == "" {
		return ""
	}
	if s.publicBaseURL != "" {
		return fmt.Sprintf("%s/%s", s.publicBaseURL, key)
	}
	return fmt.Sprintf("%s/%s/%s", s.endpointURL, s.bucket, key)
}

// HasPublicURL returns true when the endpoint URL is configured (public access possible).
func (s *S3Client) HasPublicURL() bool {
	return !s.privateBucket && (s.publicBaseURL != "" || s.endpointURL != "")
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
	if publicRead && !s.privateBucket {
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
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
		Body:        body,
	}
	if size >= 0 {
		input.ContentLength = aws.Int64(size)
	}
	if publicRead && !s.privateBucket {
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

// GeneratePresignedInlineGetURL generates a presigned GET URL suitable for inline rendering.
func (s *S3Client) GeneratePresignedInlineGetURL(key string) (string, error) {
	input := &s3.GetObjectInput{
		Bucket:                     aws.String(s.bucket),
		Key:                        aws.String(key),
		ResponseContentDisposition: aws.String("inline"),
	}

	result, err := s.presignClient.PresignGetObject(context.Background(), input, func(o *s3.PresignOptions) {
		o.Expires = 1 * time.Hour
	})
	if err != nil {
		return "", fmt.Errorf("generate presigned inline GET URL: %w", err)
	}
	return result.URL, nil
}

// GetObject downloads an object from S3 and returns its contents.
func (s *S3Client) GetObject(ctx context.Context, key string) ([]byte, error) {
	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("get S3 object: %w", err)
	}
	defer result.Body.Close()
	data, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, fmt.Errorf("read S3 object: %w", err)
	}
	return data, nil
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

// ConfigureAssetAccess keeps imported Docs media authenticated. Private bucket
// mode uses presigned requests, never ACLs, and exposes only public asset keys.
func (s *S3Client) ConfigureAssetAccess(appBaseURL string, privateBucket bool) {
	if s == nil {
		return
	}
	s.appBaseURL = strings.TrimRight(appBaseURL, "/")
	s.privateBucket = privateBucket
}

func (s *S3Client) PrivateBucket() bool { return s != nil && s.privateBucket }

func PublicAssetKey(key string) bool {
	if key == "" || path.Clean(key) != key || strings.ContainsAny(key, "\\%?#") {
		return false
	}
	parts := strings.Split(key, "/")
	if len(parts) < 4 || parts[1] == "" {
		return false
	}
	return parts[0] == "helpcenter" || (parts[0] == "users" && parts[2] == "avatar") || (parts[0] == "workspaces" && parts[2] == "logo")
}

func DocsImageKey(key, workspaceID string) bool {
	return workspaceID != "" && strings.HasPrefix(key, "docs-import/"+workspaceID+"/") && path.Clean(key) == key && !strings.ContainsAny(key, "\\%?#")
}

// DocsImageKeyFromURL recognizes only this deployment's authenticated asset URL.
func (s *S3Client) DocsImageKeyFromURL(raw, workspaceID string) (string, bool) {
	u, err := url.Parse(raw)
	if err != nil || s.appBaseURL == "" {
		return "", false
	}
	base, err := url.Parse(s.appBaseURL)
	if err != nil || u.Scheme != base.Scheme || u.Host != base.Host || u.Path != "/api/docs/images/content" {
		return "", false
	}
	key := u.Query().Get("key")
	return key, u.Query().Get("workspace_id") == workspaceID && DocsImageKey(key, workspaceID)
}
