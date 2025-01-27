package fileobject

import (
	"context"
	"io"
)

// FileObject defines the interface for file storage operations.
type FileObject interface {
	// UploadFile uploads a file to the storage and returns the file URL.
	UploadFile(ctx context.Context, bucketName, fileName string, file io.Reader) (string, error)

	// GetFileURL retrieves the public URL of a file.
	GetFileURL(bucketName, fileName string) string
}
