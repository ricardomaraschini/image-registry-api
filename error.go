package registry

import (
	"encoding/json"
	"net/http"
)

// ErrUnauthorized is used when a client attempts to execute an operation in the registry
// without or with invalid credentials.
var ErrUnauthorized = &Error{
	Status:  http.StatusUnauthorized,
	Code:    "UNAUTHORIZED",
	Message: "server does not support unauthorized requests",
}

// ErrUnknownBlob is returned to the client when it attempts to read a blob the registry
// is not aware of.
var ErrUnknownBlob = &Error{
	Status:  http.StatusNotFound,
	Code:    "BLOB_UNKNOWN",
	Message: "unknown blob",
}

// ErrUnknownManifest is returned to the client when it attempts to read a manifest the
// registry is not aware of.
var ErrUnknownManifest = &Error{
	Status:  http.StatusNotFound,
	Code:    "MANIFEST_UNKNOWN",
	Message: "unknown manifest",
}

// ErrUnsupported is returned to the client attempts to execute an http request that the
// registry does not know how to handle or hasn't it implemented yet.
var ErrUnsupported = &Error{
	Status:  http.StatusMethodNotAllowed,
	Code:    "UNSUPPORTED",
	Message: "unsupported operation",
}

// ErrInternal wraps a regular go error into a Error struct and returns it.
func ErrInternal(err error) *Error {
	return &Error{
		Status:  http.StatusInternalServerError,
		Code:    "INTERNAL_SERVER_ERROR",
		Message: err.Error(),
	}
}

// Error is used when returning errors to the runtime calling the registry API. Status refers to
// the http status code, Code follows [1] and Message is a descriptibe message.
//
// [1] https://github.com/opencontainers/distribution-spec/blob/main/spec.md#error-codes
type Error struct {
	Status  int
	Code    string
	Message string
}

// Write writes down the error (marshaled as a json) into provided ResponseWriter.
func (r *Error) Write(resp http.ResponseWriter) error {
	resp.WriteHeader(r.Status)
	return json.NewEncoder(resp).Encode(
		map[string]interface{}{
			"errors": []map[string]interface{}{
				{
					"code":    r.Code,
					"message": r.Message,
				},
			},
		},
	)
}
