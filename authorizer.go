package main

import (
	"context"
	"strings"

	"github.com/ricardomaraschini/image-registry-api/registry"
)

// Authorizer is our example implementation of an authentication mechanism.
type Authorizer struct{}

// Authenticate authenticates an user using provided Request. Returns a token or an error.
func (a *Authorizer) Authenticate(
	ctx context.Context, request registry.Request,
) (string, *registry.Error) {
	// the scope of the access can be obtained by calling:
	// scope, err := request.AccessScope()

	user, pass := request.BasicAuth()
	if user == "user" && pass == "123" {
		return "token123", nil
	}
	return "", registry.ErrUnauthorized
}

// Authorize validates the token present in the request.
func (a *Authorizer) Authorize(ctx context.Context, request registry.Request) *registry.Error {
	authorization := request.Header.Get("authorization")
	token := strings.TrimPrefix(authorization, "Bearer ")
	if token == "token123" {
		return nil
	}
	return registry.ErrUnauthorized
}
