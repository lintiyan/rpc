package server

import (
	"context"
	"distribute/rpc/protocol"
	"distribute/rpc/registry"
	"distribute/rpc/transport"
	"encoding/json"
	"sync/atomic"
)

type ServeFunc func(network, addr string) error
type TransportFunc func(conn transport.Transport)
type DoHandleRequestFunc func(ctx context.Context, request *protocol.Message, response *protocol.Message, tr transport.Transport)

type Wrapper interface {
	WrapperServe(*SGServer, ServeFunc) ServeFunc
	WrapperTransport(*SGServer, TransportFunc) TransportFunc
	WrapperDoHandleRequest(*SGServer, DoHandleRequestFunc) DoHandleRequestFunc
}

type DefaultWrapper struct{}

func (d DefaultWrapper) WrapperServe(s *SGServer, serve ServeFunc) ServeFunc {

	return func(network, addr string) error {
		// 服务注册
		services, _ := json.Marshal(s.Services())
		provider := registry.Provider{
			ProviderKey: network + "@" + addr,
			Network:     network,
			Addr:        addr,
			Meta:        map[string]string{"services": string(services)},
		}

		registry := s.option.Registry
		registry.Register(s.option.RegistryOption, provider)

		return serve(network, addr)
	}
}

func (d DefaultWrapper) WrapperTransport(s *SGServer, tr TransportFunc) TransportFunc {
	return tr
}

func (d DefaultWrapper) WrapperDoHandleRequest(s *SGServer, hr DoHandleRequestFunc) DoHandleRequestFunc {
	return func(ctx context.Context, request *protocol.Message, response *protocol.Message, tr transport.Transport) {
		atomic.AddInt64(&s.RequestInProcess, 1)
		hr(ctx, request, response, tr)
		atomic.AddInt64(&s.RequestInProcess, -1)
	}
}
