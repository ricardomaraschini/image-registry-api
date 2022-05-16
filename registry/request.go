package registry

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
)

// AccessScope holds information about the scope of a given http access executed agains the
// registry. This information is passed to us by means of GET variables in the URL during
// requests to /auth endpoint.
type AccessScope struct {
	Account string
	Scope   Scope
	Service string
}

// Scope holds the scope of a http access. Image holds the repository/image pair while the
// operations holds the type of operation (pull, push).
type Scope struct {
	Repository string
	Image      string
	Operations []string
}

// Request wraps a default http.Request reference. Provides some tooling around analysing the
// desired intent of the embed http.Request. Registry protocol is a huge mess, it is easir to
// gather all url related parsing and foo into a single entity.
type Request struct {
	*http.Request
}

// BasicAuth parses the Basic authentication sent by the container runtime in a header named
// authorization. This function does not return errors, if the information could not be parsed
// empty strings are returned.
func (r *Request) BasicAuth() (string, string) {
	authorization := r.Header.Get("authorization")
	if len(authorization) == 0 {
		return "", ""
	}

	if !strings.HasPrefix(authorization, "Basic ") {
		return "", ""
	}

	authorization = strings.TrimPrefix(authorization, "Basic ")
	decoded, err := base64.StdEncoding.DecodeString(authorization)
	if err != nil {
		return "", ""
	}

	userpass := string(decoded)
	slices := strings.SplitN(userpass, ":", 2)
	if len(slices) != 2 {
		return "", ""
	}
	return slices[0], slices[1]
}

// AccessScope extracts the access scope (as sent by the container runtime) from the request.
func (r *Request) AccessScope() (*AccessScope, error) {
	// scope format is "repository:reponame/imagename:operation-0,operation-1", we need to
	// parse this info and add it to the AccessScope.
	rscope := strings.Split(r.Get("scope"), ":")
	if len(rscope) != 3 {
		return nil, fmt.Errorf("invalid authentication scope")
	}

	operations := strings.Split(rscope[2], ",")
	repoAndImage := strings.Split(rscope[1], "/")
	if len(repoAndImage) != 2 {
		return nil, fmt.Errorf("invalid scope repository/image")
	}

	return &AccessScope{
		Account: r.Get("account"),
		Service: r.Get("service"),
		Scope: Scope{
			Image:      repoAndImage[1],
			Repository: repoAndImage[0],
			Operations: operations,
		},
	}, nil
}

// Get extracts and returns a Get variable from the inner request.
func (r *Request) Get(gvar string) string {
	return r.Request.URL.Query().Get(gvar)
}

// IsPing verifies if the request points to /v2 or /v2/ path. This is the url used by container
// runtime when it needs to verify if it can reach thre registry or not.
func (r *Request) IsPing() bool {
	turl := strings.TrimSuffix(r.Request.URL.Path, "/")
	return turl == "/v2"
}

// IsAuth verifies if the url path points to our authentication endpoint. The authentication
// endpoint path is "/v2/auth".
func (r *Request) IsAuth() bool {
	turl := strings.TrimSuffix(r.Request.URL.Path, "/")
	return turl == "/v2/auth"
}

// IsBlob returns true if the url refers to a blob access.
func (r *Request) IsBlob() bool {
	return strings.Contains(r.Request.URL.Path, "/blobs/")
}

// IsBlobUploadRequest returns true if the url refers to a request to start uploading a blob.
func (r *Request) IsBlobUploadRequest() bool {
	turl := strings.TrimSuffix(r.Request.URL.Path, "/")
	return strings.HasSuffix(turl, "/blobs/uploads")
}

// IsHead returns true if this is an http.MethodHead request.
func (r *Request) IsHead() bool {
	return r.Request.Method == http.MethodHead
}

// IsPatch returns true if this is an http.MethodPatch request.
func (r *Request) IsPatch() bool {
	return r.Request.Method == http.MethodPatch
}

// IsGet returns true if this is an http.MethodGet request.
func (r *Request) IsGet() bool {
	return r.Request.Method == http.MethodGet
}

// IsPut returns true if this is an http.MethodPut request.
func (r *Request) IsPut() bool {
	return r.Request.Method == http.MethodPut
}

// IsDelete returns true if this is an http.MethodDelete request.
func (r *Request) IsDelete() bool {
	return r.Request.Method == http.MethodDelete
}

// HasBlobUploadID returns true if the url contains an upload identification, this generally
// means that a client is uploading blob data.
func (r *Request) HasBlobUploadID() bool {
	return strings.Contains(r.Request.URL.Path, "/blobs/upload/id/")
}

// RepositoryAndImage attempts to extract repository and image references from the inner req,
// the url format is expected to be like /v2/<repository>/<image>/...
func (r *Request) RepositoryAndImage() (string, string, error) {
	parts := strings.Split(r.Request.URL.Path, "/")
	if len(parts) < 4 {
		return "", "", fmt.Errorf("unable to extract url repository and image")
	}
	return parts[2], parts[3], nil
}

// ContentType returns the content type header from the inner request.
func (r *Request) ContentType() string {
	return r.Request.Header.Get("content-type")
}

// IsManifest returns true if the url refers to a manifest access.
func (r *Request) IsManifest() bool {
	return strings.Contains(r.Request.URL.Path, "/manifests/")
}

// last splits the underlying request path and returns the last component. If the underlying url
// path is just "/" returns an empty string.
func (r *Request) last() string {
	parts := strings.Split(r.Request.URL.Path, "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

// UploadID extracts the upload id from the url.
func (r *Request) UploadID() string {
	return r.last()
}

// BlobHash extracts the blob hash from the  underlying url.
func (r *Request) BlobHash() string {
	return r.last()
}

// ManifestID extracts the manifst tag or hash from the  underlying url.
func (r *Request) ManifestID() string {
	return r.last()
}
