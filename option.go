package registry

// Option is a function that sets an Option in a Registry reference.
type Option func(*Registry)

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
