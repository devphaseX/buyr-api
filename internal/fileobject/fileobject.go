package fileobject

import (
	"context"
	"io"
)

const (
	_  = iota             // 0
	KB = 1 << (10 * iota) // 1 << 10 = 1024
	MB                    // 1 << 20 = 1,048,576
)

// FileObject defines the interface for file storage operations.
type FileObject interface {
	// UploadFile uploads a file to the storage and returns the file URL.
	UploadFile(ctx context.Context, bucketName, fileName string, file io.Reader) (string, error)

	// GetFileURL retrieves the public URL of a file.
	GetFileURL(bucketName, fileName string) string
}
