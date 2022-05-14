package main

import (
	"net/http"

	"github.com/ricardomaraschini/image-registry-api/registry"
)

func main() {
	authzer := &Authorizer{}
	reghandler := registry.New(authzer)

	err := http.ListenAndServeTLS(":8080", "certs/server.crt", "certs/server.key", reghandler)
	if err != nil {
		panic(err)
	}
}
