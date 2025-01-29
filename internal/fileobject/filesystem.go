package fileobject

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// FileSystemStorage implements the FileObject interface for local file storage.
type FileSystemStorage struct {
	BasePath string // Base directory where files will be stored
}

// UploadFile uploads a file to the local file system.
func (fs *FileSystemStorage) UploadFile(ctx context.Context, bucketName, fileName string, file io.Reader) (string, error) {
	// Create the bucket directory if it doesn't exist
	bucketPath := filepath.Join(fs.BasePath, bucketName)
	if err := os.MkdirAll(bucketPath, os.ModePerm); err != nil {
		return "", fmt.Errorf("failed to create bucket directory: %w", err)
	}

	// Create the file on the local file system
	filePath := filepath.Join(bucketPath, fileName)
	outFile, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer outFile.Close()

	// Copy the file content to the new file
	if _, err := io.Copy(outFile, file); err != nil {
		return "", fmt.Errorf("failed to copy file content: %w", err)
	}

	// Return the file URL
	return fs.GetFileURL(bucketName, fileName), nil
}

// GetFileURL retrieves the public URL of a file.
func (fs *FileSystemStorage) GetFileURL(bucketName, fileName string) string {
	return fmt.Sprintf("/%s/%s", bucketName, fileName)
}
