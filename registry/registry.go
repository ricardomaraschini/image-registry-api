package registry

import (
	"encoding/json"
	"fmt"
	"net/http"

	"k8s.io/klog"
)

// Authorizer is an abstraction so we users can provide their own implementation. Two functions
// are required here: Authenticate receives a request to authenticate a user and returns a token
// or and Error while Authorize validates the token and returns an error if invalid or nil if
// the token is valid.
type Authorizer interface {
	Authenticate(Request) (string, *Error)
	Authorize(Request) *Error
}

// Registry is our middleware to access the backend registry. This object implements an http
// Handler and dispatches all received requests directly to our backend registry. This entity
// also manages users authentication.
type Registry struct {
	srvaddr string
	blobhdr *BlobHandler
	manfhdr *ManifestHandler
	authzer Authorizer
}

// redirectToAuth redirect the client do the authentication endpoint by means of setting the
// 'www-authenticate' header value to the appropriate url. if no authorization header is
// present this function replies requests with unauthorized.
func (r *Registry) redirectToAuth(resp http.ResponseWriter, request Request) {
	resp.Header().Add("docker-distribution-api-version", "registry/2.0")
	if err := r.authzer.Authorize(request); err == nil {
		resp.WriteHeader(http.StatusOK)
		return
	}

	realm := fmt.Sprintf("https://%s/v2/auth", r.srvaddr)
	authdr := fmt.Sprintf("bearer realm=\"%s\",service=\"%s\"", realm, r.srvaddr)
	resp.Header().Add("www-authenticate", authdr)
	resp.WriteHeader(http.StatusUnauthorized)
}

// authenticate manages the user authentication.
func (r *Registry) authenticate(resp http.ResponseWriter, request Request) {
	resp.Header().Add("docker-distribution-api-version", "registry/2.0")
	resp.Header().Add("content-type", "application/json")

	token, err := r.authzer.Authenticate(request)
	if err != nil {
		err.Write(resp)
		return
	}

	content := map[string]string{"token": token}
	if err := json.NewEncoder(resp).Encode(content); err != nil {
		klog.Errorf("error encoding token: %s", err)
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
	if err := r.authzer.Authorize(request); err != nil {
		err.Write(resp)
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

// New returns a http handler for our image registry requests.
func New(auth Authorizer) http.Handler {
	sthandler := NewStorageHandler()
	return &Registry{
		srvaddr: "localhost:8080",
		blobhdr: NewBlobHandler(sthandler),
		manfhdr: NewManifestHandler(sthandler),
		authzer: auth,
	}
}
