package registry

// Option is a function that sets an Option in a Registry reference.
type Option func(*Registry)

// WithExternalAddress sets the external address. External address is used by the registry api
// when redirecting clients (container runtime) to the correct authentication endpoint.
func WithExternalAddress(addr string) Option {
	return func(r *Registry) {
		r.externaladdr = addr
	}
}

// WithCert sets the certificate and key to be used by the registry api.
func WithCert(certpath, keypath string) Option {
	return func(r *Registry) {
		r.certpath = certpath
		r.keypath = keypath
	}
}

// WithBindAddress sets the bind address for the http server.
func WithBindAddress(addr string) Option {
	return func(r *Registry) {
		r.bind = addr
	}
}

// WithEventHandler adds provided event handler to the registry
func WithEventHandler(eh EventHandler) Option {
	return func(r *Registry) {
		r.manfhdr.evthandler = eh
	}
}
