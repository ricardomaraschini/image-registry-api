package registry

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/containers/image/v5/manifest"
	"k8s.io/klog"
)

// ManifestTag is used when storing a manifest tag in our storage layer.
type ManifestTag struct {
	Hash        string `json:"hash"`
	ContentType string `json:"contentType"`
}

// ManifestHandler handles all manifest related operations.
type ManifestHandler struct {
	storage    *StorageHandler
	evthandler EventHandler
}

// StoreManifest stores a manifest in our underlying storage.
func (m *ManifestHandler) StoreManifest(resp http.ResponseWriter, request Request) {
	manid := request.ManifestID()
	repo, image, err := request.RepositoryAndImage()
	if err != nil {
		klog.Errorf("error parsing repo/image: %s", err)
		ErrInternal(err).Write(resp)
		return
	}

	hasher := sha256.New()
	buf := bytes.NewBuffer(nil)
	to := io.MultiWriter(buf, hasher)
	if _, err := io.Copy(to, request.Body); err != nil {
		klog.Errorf("error copying manifest blob: %s", err)
		ErrInternal(err).Write(resp)
		return
	}

	hash := fmt.Sprintf("sha256:%x", hasher.Sum(nil))
	if err := m.storage.PutBlob(repo, image, hash, buf); err != nil {
		klog.Errorf("error saving manifest blob: %s", err)
		ErrInternal(err).Write(resp)
		return
	}

	if strings.HasPrefix(manid, "sha256:") {
		klog.Infof("new manifest upload %s/%s@%s", repo, image, manid)
		resp.WriteHeader(http.StatusCreated)
		return
	}

	if err := m.storage.PutTag(repo, image, manid, hash); err != nil {
		klog.Errorf("error saving manifest tag file: %s", err)
		ErrInternal(err).Write(resp)
		return
	}

	if m.evthandler != nil {
		err := m.evthandler.NewTag(request.Context(), repo, image, manid)
		if err != nil {
			klog.Errorf("event handler failed: %s", err)
			ErrInternal(err).Write(resp)
			return
		}
	}

	klog.Infof("new manifest tag upload %s/%s:%s", repo, image, manid)
	resp.Header().Set("docker-content-digest", hash)
	resp.WriteHeader(http.StatusCreated)
}

// GetManifest returns a manifest from the storage. Reference to the manifest may be made by
// means of a tag ("latest" for instance) or by the manifest hash (sha256).
func (m *ManifestHandler) GetManifest(resp http.ResponseWriter, request Request) {
	manid := request.ManifestID()
	repo, image, err := request.RepositoryAndImage()
	if err != nil {
		klog.Errorf("error parsing image/repo for upload: %s", err)
		ErrInternal(err).Write(resp)
		return
	}

	var manread io.ReadCloser
	var mansize int64
	if strings.HasPrefix(manid, "sha256:") {
		manread, mansize, err = m.storage.GetBlob(repo, image, manid)
	} else {
		manread, mansize, err = m.storage.GetTag(repo, image, manid)
	}

	if err != nil {
		if err := errors.Unwrap(err); os.IsNotExist(err) {
			ErrUnknownManifest.Write(resp)
			return
		}
		klog.Errorf("error getting manifest blob: %s", err)
		ErrInternal(err).Write(resp)
		return
	}
	defer manread.Close()

	mandata, err := io.ReadAll(manread)
	if err != nil {
		klog.Errorf("error reading manifest blob: %s", err)
		ErrInternal(err).Write(resp)
		return
	}

	resp.Header().Add("content-length", fmt.Sprint(mansize))
	resp.Header().Add("content-type", manifest.GuessMIMEType(mandata))
	resp.Header().Add("content-type", "application/json")
	resp.Write(mandata)
}

// ServeHTTP is our http handler for manifest related requests.
func (m *ManifestHandler) ServeHTTP(resp http.ResponseWriter, request Request) {
	switch {
	case request.IsGet():
		m.GetManifest(resp, request)
	case request.IsPut():
		m.StoreManifest(resp, request)
	default:
		ErrUnsupported.Write(resp)
	}
}

// NewManifestHandler returns a new http handler manifest related operations.
func NewManifestHandler(handler *StorageHandler) *ManifestHandler {
	return &ManifestHandler{
		storage: handler,
	}
}
