package registry

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"k8s.io/klog"
)

// NewBlobHandler returns a new http handler for blob operations.
func NewBlobHandler(sthandler *StorageHandler) *BlobHandler {
	return &BlobHandler{
		upload:  NewUploadHandler(),
		storage: sthandler,
	}
}

// BlobHandler handles all blob related operations.
type BlobHandler struct {
	upload  *UploadHandler
	storage *StorageHandler
}

// Stat verifies if the blob already exists in our storage.
func (b *BlobHandler) Stat(resp http.ResponseWriter, request Request) {
	repo, img, err := request.RepositoryAndImage()
	if err != nil {
		klog.Errorf("error fetching repo/image: %s", err)
		ErrInternal(err).Write(resp)
		return
	}

	hash := request.BlobHash()
	size, err := b.storage.StatBlob(repo, img, hash)
	if err != nil && !os.IsNotExist(err) {
		klog.Errorf("unable to stat blob: %s", err)
		ErrInternal(err).Write(resp)
		return
	}

	if os.IsNotExist(err) {
		ErrUnknownBlob.Write(resp)
		return
	}

	trimhash := strings.TrimPrefix(hash, "sha256:")
	resp.Header().Set("content-length", fmt.Sprint(size))
	resp.Header().Set("docker-content-digest", trimhash)
	resp.WriteHeader(http.StatusOK)
}

// StartBlobUpload returns a temporary url where a blob upload can take place. Return a
// Location header to be followed by the client when uploading the blob.
func (b *BlobHandler) StartBlobUpload(resp http.ResponseWriter, request Request) {
	repo, img, err := request.RepositoryAndImage()
	if err != nil {
		klog.Errorf("error parsing image/repo for upload: %s", err)
		ErrInternal(err).Write(resp)
		return
	}

	id := b.upload.Start(20 * time.Minute)
	newloc := fmt.Sprintf("/v2/%s/%s/blobs/upload/id/%s", repo, img, id)
	resp.Header().Set("location", newloc)
	resp.Header().Set("range", "0-0")
	resp.WriteHeader(http.StatusAccepted)
}

// Get returns a blob by its hash (sha256).
func (b *BlobHandler) Get(resp http.ResponseWriter, request Request) {
	hash := request.BlobHash()
	repo, image, err := request.RepositoryAndImage()
	if err != nil {
		klog.Errorf("unable to parse repo/image: %s", err)
		ErrInternal(err).Write(resp)
		return
	}

	fp, fsize, err := b.storage.GetBlob(repo, image, hash)
	if err != nil {
		if err := errors.Unwrap(err); os.IsNotExist(err) {
			ErrUnknownBlob.Write(resp)
			return
		}
		klog.Errorf("unable to get blob: %s", err)
		ErrInternal(err).Write(resp)
		return
	}
	defer fp.Close()

	resp.Header().Add("content-length", fmt.Sprint(fsize))
	if _, err := io.Copy(resp, fp); err != nil {
		klog.Errorf("error copying blob: %s", err)
	}
}

// UploadBlob manages blob upload requests. This function is called when there is something
// being uploaded by the client. We expect to find a valid upload 'id' in the url.
func (b *BlobHandler) UploadBlob(resp http.ResponseWriter, request Request) {
	id := request.UploadID()
	if len(id) == 0 {
		err := fmt.Errorf("empty upload id")
		klog.Errorf("invalid request: %s", err)
		ErrInternal(err).Write(resp)
		return
	}

	repo, img, err := request.RepositoryAndImage()
	if err != nil {
		klog.Errorf("unable to parse repo/image: %s", err)
		ErrInternal(err).Write(resp)
		return
	}

	if request.IsDelete() {
		b.upload.Delete(id)
		resp.WriteHeader(http.StatusOK)
		return
	}

	written, err := b.upload.Append(id, request.Body)
	if err != nil {
		klog.Errorf("error append to upload file: %s", err)
		ErrInternal(err).Write(resp)
		return
	}

	newloc := fmt.Sprintf("/v2/%s/%s/blobs/upload/id/%s", repo, img, id)
	resp.Header().Set("location", newloc)
	resp.Header().Set("range", fmt.Sprintf("0-%d", written))

	if request.IsPatch() {
		// if the method is patch we still expect more slices of bytes coming our way
		// during the next requests, just return StatusNoContent.
		resp.WriteHeader(http.StatusNoContent)
		return
	}

	fp, err := b.upload.End(id)
	if err != nil {
		klog.Errorf("unable to commit uploaded file: %s", err)
		ErrInternal(err).Write(resp)
		return
	}
	defer fp.Close()

	expdgst := request.Get("digest")
	if expdgst == "" {
		err := fmt.Errorf("empty digest provided during upload")
		klog.Errorf("invalid request: %s", err)
		ErrInternal(err).Write(resp)
		return
	}

	if err := b.storage.PutBlob(repo, img, expdgst, fp); err != nil {
		klog.Errorf("error commiting blob to storage: %s", err)
		ErrInternal(err).Write(resp)
	}
	klog.Infof("new blob upload %s/%s@%s", repo, img, expdgst)
	resp.WriteHeader(http.StatusCreated)
}

func (b *BlobHandler) ServeHTTP(resp http.ResponseWriter, request Request) {
	switch {
	case request.IsHead():
		b.Stat(resp, request)
	case request.IsGet():
		b.Get(resp, request)
	case request.HasBlobUploadID():
		b.UploadBlob(resp, request)
	case request.IsBlobUploadRequest():
		b.StartBlobUpload(resp, request)
	default:
		ErrUnsupported.Write(resp)
	}
}
