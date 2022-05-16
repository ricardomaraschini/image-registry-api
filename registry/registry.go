package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"k8s.io/klog"
)

// Authorizer is an abstraction so we users can provide their own implementation. Two functions
// are required here: Authenticate receives a request to authenticate a user and returns a token
// or and Error while Authorize validates the token and returns an error if invalid or nil if
// the token is valid.
type Authorizer interface {
	Authenticate(context.Context, Request) (string, *Error)
	Authorize(context.Context, Request) *Error
}

// EventHandler is implmemented by any entity observing events in the registry.
type EventHandler interface {
	NewTag(context.Context, string, string, string) error
}

// Registry is our middleware to access the backend registry. This object implements an http
// Handler and dispatches all received requests directly to our backend registry. This entity
// also manages users authentication.
type Registry struct {
	blobhdr    *BlobHandler
	manfhdr    *ManifestHandler
	authzer    Authorizer
	certpath   string
	keypath    string
	bind       string
	evthandler EventHandler
}

// redirectToAuth redirect the client do the authentication endpoint by means of setting the
// 'www-authenticate' header value to the appropriate url. if no authorization header is
// present this function replies requests with unauthorized.
func (r *Registry) redirectToAuth(resp http.ResponseWriter, request Request) {
	resp.Header().Add("docker-distribution-api-version", "registry/2.0")
	if err := r.authzer.Authorize(request.Context(), request); err == nil {
		resp.WriteHeader(http.StatusOK)
		return
	}

	realm := fmt.Sprintf("https://%s/v2/auth", request.Host)
	authdr := fmt.Sprintf("bearer realm=\"%s\",service=\"%s\"", realm, request.Host)
	resp.Header().Add("www-authenticate", authdr)
	resp.WriteHeader(http.StatusUnauthorized)
}

// authenticate manages the user authentication.
func (r *Registry) authenticate(resp http.ResponseWriter, request Request) {
	resp.Header().Add("docker-distribution-api-version", "registry/2.0")
	resp.Header().Add("content-type", "application/json")

	token, err := r.authzer.Authenticate(request.Context(), request)
	if err != nil {
		err.Write(resp)
		klog.Errorf("unable to authenticate user: %q", err.Message)
		return
	}

	content := map[string]string{"token": token}
	if err := json.NewEncoder(resp).Encode(content); err != nil {
		klog.Errorf("error encoding token: %q", err)
	}
}

// ServeHTTP is our main http handler. Attempts to understand the request and dispatches to
// the appropriate handler.
func (r *Registry) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	request := Request{req}
	if request.IsPing() {
		r.redirectToAuth(resp, request)
		return
	}
	if request.IsAuth() {
		r.authenticate(resp, request)
		return
	}
	if err := r.authzer.Authorize(request.Context(), request); err != nil {
		err.Write(resp)
		klog.Errorf("unable to authorize token: %q", err.Message)
		return
	}
	if request.IsBlob() {
		r.blobhdr.ServeHTTP(resp, request)
		return
	}
	if request.IsManifest() {
		r.manfhdr.ServeHTTP(resp, request)
		return
	}
	ErrUnsupported.Write(resp)
}

// Start puts the metrics http server online.
func (r *Registry) Start(ctx context.Context) error {
	server := &http.Server{
		Addr:    r.bind,
		Handler: r,
	}

	go func() {
		<-ctx.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			klog.Errorf("error shutting down https server: %s", err)
		}
	}()

	var wg sync.WaitGroup
	wg.Add(1)
	go r.blobhdr.upload.gc(ctx, &wg)

	if err := server.ListenAndServeTLS("certs/server.crt", "certs/server.key"); err != nil {
		wg.Wait()
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
	wg.Wait()
	return nil
}

// New returns a http handler for our image registry requests.
func New(auth Authorizer, opts ...Option) *Registry {
	sthandler := NewStorageHandler()
	registry := &Registry{
		bind:     ":8080",
		certpath: "certs/server.crt",
		keypath:  "certs/server.key",
		blobhdr:  NewBlobHandler(sthandler),
		manfhdr:  NewManifestHandler(sthandler),
		authzer:  auth,
	}

	for _, opt := range opts {
		opt(registry)
	}
	return registry
}
