package build

import "github.com/SeaArt-Infra/sandbox-go/core"

// Service wraps the shared core transport with template/build APIs.
type Service struct {
	*core.Transport
}

// NewService creates a build-plane service using the shared core transport.
func NewService(baseURL, apiKey string, opts ...core.TransportOption) (*Service, error) {
	base, err := core.NewTransport(baseURL, apiKey, opts...)
	if err != nil {
		return nil, err
	}
	return &Service{Transport: base}, nil
}
