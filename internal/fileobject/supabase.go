package fileobject

import (
	"context"
	"fmt"
	"io"

	storage_go "github.com/supabase-community/storage-go"
)

// SupabaseStorage implements the FileObject interface for Supabase storage.
type SupabaseStorage struct {
	client *storage_go.Client
}

// NewSupabaseStorage creates a new SupabaseStorage instance.
func NewSupabaseStorage(supabaseURL, supabaseKey string) (*SupabaseStorage, error) {
	client := storage_go.NewClient(supabaseURL, supabaseKey, nil)
	return &SupabaseStorage{client: client}, nil
}

// UploadFile uploads a file to Supabase storage and returns the file URL.
func (s *SupabaseStorage) UploadFile(ctx context.Context, bucketName, fileName string, file io.Reader) (string, error) {
	// Upload the file to the specified bucket
	_, err := s.client.UploadFile(bucketName, fileName, file)

	if err != nil {

		return "", fmt.Errorf("failed to upload file: %w", err)
	}

	// Return the public URL of the uploaded file
	return s.GetFileURL(bucketName, fileName), nil
}

// GetFileURL retrieves the public URL of a file in Supabase storage.
func (s *SupabaseStorage) GetFileURL(bucketName, fileName string) string {
	return s.client.GetPublicUrl(bucketName, fileName).SignedURL
}

// DeleteFile deletes a file from Supabase storage.
func (s *SupabaseStorage) DeleteFile(ctx context.Context, bucketName, fileName string) error {
	_, err := s.client.RemoveFile(bucketName, []string{fileName})
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}
	return nil
}
