package registry

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
)

// StorageHandler manages our on disk blob storage.
type StorageHandler struct {
	basedir string
}

// PutTag stores a manifest tag. The tag is stored in the 'tags' directory and it is a regular
// file whose content is the blob name where the manifest for the tag is stored.
func (s *StorageHandler) PutTag(repo, image, tag, hash string) error {
	tagdir := fmt.Sprintf("%s/%s/%s/tags", s.basedir, repo, image)
	if err := os.MkdirAll(tagdir, os.ModePerm); err != nil && !os.IsExist(err) {
		return fmt.Errorf("unable to create manifest storage: %w", err)
	}

	tagpath := fmt.Sprintf("%s/%s", tagdir, tag)
	manfp, err := os.OpenFile(tagpath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("unable to create tag file: %w", err)
	}
	defer manfp.Close()

	if _, err := manfp.WriteString(hash); err != nil {
		return fmt.Errorf("unable to write to tag file: %w", err)
	}
	return nil
}

// GetTag gets a manifest tag. Reads the tag file then attempts to read the blob where the
// manifest is stored. Returns a ReadCloser from where the manifest can be read. It is caller
// responsibility to close the returned ReadCloser.
func (s *StorageHandler) GetTag(repo, image, tag string) (io.ReadCloser, int64, error) {
	tagpath := fmt.Sprintf("%s/%s/%s/tags/%s", s.basedir, repo, image, tag)
	data, err := os.ReadFile(tagpath)
	if err != nil {
		return nil, 0, fmt.Errorf("unable to read tag file: %w", err)
	}

	hash := string(data)
	return s.GetBlob(repo, image, hash)
}

// GetBlob gets a blob from our storage. Returns a ReadCloser from where the blob content can be
// read and it caller's responsibility to close the returned ReadCloser.
func (s *StorageHandler) GetBlob(repo, image, hash string) (io.ReadCloser, int64, error) {
	blobpath := fmt.Sprintf("%s/%s/%s/%s", s.basedir, repo, image, hash)
	blobfp, err := os.Open(blobpath)
	if err != nil {
		return nil, 0, fmt.Errorf("unable to open blob file: %w", err)
	}

	finfo, err := blobfp.Stat()
	if err != nil {
		blobfp.Close()
		return nil, 0, fmt.Errorf("unable to read blob properties: %w", err)
	}

	return blobfp, finfo.Size(), nil
}

// PutBlob writes content from the provided io.Reader as a blob of the provided repository
// and image pair. Checks if the written hash matches the provided hash and returns an error
// if there is a mismatch. In case of mismatch the file is deleted from disk.
func (s *StorageHandler) PutBlob(repo, image, hash string, from io.Reader) error {
	repodir := fmt.Sprintf("%s/%s/%s", s.basedir, repo, image)
	if err := os.MkdirAll(repodir, os.ModePerm); err != nil && !os.IsExist(err) {
		return fmt.Errorf("unable to create image storage: %w", err)
	}

	blobpath := fmt.Sprintf("%s/%s/%s/%s", s.basedir, repo, image, hash)
	blobfp, err := os.OpenFile(blobpath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("unable to create blob file: %w", err)
	}
	defer blobfp.Close()

	hasher := sha256.New()
	to := io.MultiWriter(blobfp, hasher)
	if _, err := io.Copy(to, from); err != nil {
		_ = os.RemoveAll(blobpath)
		return fmt.Errorf("error copying blob: %w", err)
	}

	reshash := fmt.Sprintf("sha256:%x", hasher.Sum(nil))
	if hash != reshash {
		_ = os.RemoveAll(blobpath)
		return fmt.Errorf("blob hash mismatch")
	}
	return nil
}

// StatBlob checks if a blob identified by its hash exists inside the provided repository and
// image.
func (s *StorageHandler) StatBlob(repo, image, hash string) (int64, error) {
	fpath := fmt.Sprintf("%s/%s/%s/%s", s.basedir, repo, image, hash)
	finfo, err := os.Stat(fpath)
	if err != nil {
		return 0, err
	}
	return finfo.Size(), nil
}

// NewStorageHandler returns a new storage handler for image blobs.
func NewStorageHandler() *StorageHandler {
	return &StorageHandler{
		basedir: "/tmp/storage",
	}
}
