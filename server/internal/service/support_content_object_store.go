package service

import "context"

type supportContentObjectStore interface {
	HasPublicURL() bool
	GeneratePresignedPutURL(key, contentType string, size int64, publicRead bool) (string, error)
	GetObject(ctx context.Context, key string) ([]byte, error)
	DeleteObject(ctx context.Context, key string) error
}
