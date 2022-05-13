package registry

import (
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"k8s.io/klog"

	"github.com/google/uuid"
)

// tmpFileWrapper wraps an os.File reference and provide tooling around deleting the temporary
// file when a call to Close() is executed.
type tmpFileWrapper struct {
	*os.File
}

// Close closes the underlying os.File and removes the file from the disk.
func (t *tmpFileWrapper) Close() error {
	if err := t.File.Close(); err != nil {
		return err
	}
	return os.RemoveAll(t.File.Name())
}

// UploadHandler handles the phisical storage
type UploadHandler struct {
	sync.Mutex
	active  map[string]time.Time
	basedir string
}

// gc collects inactive upload ids and deletes their underlying files as soon as they expire, gc
// stands for garbage collection. This function also inspects the basedir for files that have no
// more active references (left overs) and removes them.
func (u *UploadHandler) gc() {
	ticker := time.NewTicker(time.Minute)
	for range ticker.C {
		u.Lock()
		for id, deadline := range u.active {
			if deadline.After(time.Now()) {
				continue
			}

			fpath := u.tmpFileForUpload(id)
			if err := os.RemoveAll(fpath); err != nil {
				klog.Errorf("unable to delete upload file: %s", err)
			}
			delete(u.active, id)
		}

		files, err := os.ReadDir(u.basedir)
		if err != nil {
			klog.Errorf("unable to list upload files: %s", err)
			u.Unlock()
			continue
		}

		for _, file := range files {
			id := u.idForUploadFile(file.Name())
			if _, ok := u.active[id]; ok {
				continue
			}

			fpath := fmt.Sprintf("%s/%s", u.basedir, file.Name())
			if err := os.RemoveAll(fpath); err != nil {
				klog.Errorf("unable to delete upload file: %s", err)
			}
		}

		u.Unlock()
	}
}

// idForUploadFile returns the id for a given file. Files are named as <id>.tmp so this function
// only splits the file path and returns the file name without extension.
func (u *UploadHandler) idForUploadFile(fpath string) string {
	_, fname := path.Split(fpath)
	return strings.TrimSuffix(fname, ".tmp")
}

// Start creates an unique id for a given upload. This function must be called to allocate an
// slot in our uploads database. As an argument caller must inform for how long they want to
// keep the slot available, after this the slot is invalidated and any dangling content is
// removed from the filesystem.
func (u *UploadHandler) Start(deadline time.Duration) string {
	u.Lock()
	defer u.Unlock()

	id := uuid.New().String()
	u.active[id] = time.Now().Add(deadline)
	return id
}

// isValid checks if the provided upload id is still active (exists and is not expired).
func (u *UploadHandler) isValid(id string) error {
	if _, err := uuid.Parse(id); err != nil {
		return fmt.Errorf("invalid upload id: %w", err)
	}

	u.Lock()
	defer u.Unlock()

	expire, ok := u.active[id]
	if !ok {
		return fmt.Errorf("unknown upload id")
	}

	if time.Now().After(expire) {
		return fmt.Errorf("upload id expired")
	}
	return nil
}

// tmpFileForUpload returns a tmp file path for the provided upload id.
func (u *UploadHandler) tmpFileForUpload(id string) string {
	return fmt.Sprintf("%s/%s.tmp", u.basedir, id)
}

// Append appends the provided Reader to the underlying upload under the provide id. Returns
// the amount of written bytes or an error. In case of error the underlying upload for the
// provided id may be left in an unknown state.
func (u *UploadHandler) Append(id string, from io.Reader) (int64, error) {
	if err := u.isValid(id); err != nil {
		return 0, fmt.Errorf("unable to append to upload: %w", err)
	}

	fpath := u.tmpFileForUpload(id)
	fp, err := os.OpenFile(fpath, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return 0, fmt.Errorf("unable to append to storage: %w", err)
	}
	defer fp.Close()

	written, err := io.Copy(fp, from)
	if err != nil {
		return 0, fmt.Errorf("unable to copy data: %w", err)
	}
	return written, nil
}

// End ends the upload identified by the provided id. Returns a ReadCloser from where the upload
// content can be read. If no error is returned then the upload with the provided id becomes not
// active. It is responsibility of the caller to call Close() on returned Closer.
func (u *UploadHandler) End(id string) (io.ReadCloser, error) {
	if err := u.isValid(id); err != nil {
		return nil, fmt.Errorf("unable to append to upload: %w", err)
	}

	fpath := u.tmpFileForUpload(id)
	fp, err := os.Open(fpath)
	if err != nil {
		return nil, fmt.Errorf("unable to access tmp file: %w", err)
	}

	u.Lock()
	delete(u.active, id)
	u.Unlock()

	return &tmpFileWrapper{fp}, nil
}

// NewUploadHandler returns a new storage handler. This storage handler is used to store upload
// content into temporary files in local filesystem.
func NewUploadHandler() *UploadHandler {
	u := &UploadHandler{
		active:  map[string]time.Time{},
		basedir: "/tmp/uploads",
	}
	go u.gc()
	return u
}
