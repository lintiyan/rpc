package client

import (
	"context"
	"distribute/rpc/registry"
	"errors"
	"fmt"
	"sync"
)

type SGClient interface {
	Go(ctx context.Context, serviceMethod string, args, reply interface{}, done chan *Call) (*Call, error)
	Call(ctx context.Context, serviceMethod string, args, reply interface{}) error
}

type sgClient struct {
	mutex    sync.RWMutex
	clients  sync.Map // map[provider_key]rpcClient , 这里是一个常备链接池，value是普通的rpc链接
	servers  []registry.Provider
	option   SGOption
	shutdown bool
}

func (s *sgClient) Go(ctx context.Context, serviceMethod string, args, reply interface{}, done chan *Call) (*Call, error) {

	if s.shutdown {
		return nil, errors.New("client is shutdown")
	}

	_, client, err := s.SelectClient(ctx, serviceMethod, args)
	if err != nil {
		return nil, err
	}

	return WrapperGo(s, client.Go)(ctx, serviceMethod, args, reply, done)
}

func WrapperGo(s *sgClient, f GoFunc) GoFunc {

	for _, wrapper := range s.option.Wrappers {
		f = wrapper.WrapperGo(s, f)
	}
	return f
}

func (s *sgClient) Call(ctx context.Context, serviceMethod string, args, reply interface{}) error {

	provider, client, err := s.SelectClient(ctx, serviceMethod, args)
	if err != nil && s.option.FailMode == FailFast {
		return err
	}
	fmt.Println("provider=", provider)
	retrys := s.option.Retries

	switch s.option.FailMode {
	case FailRetry:
		for retrys > 0 {
			retrys--
			if client != nil {
				err := WrapperCall(s, client.Call)(ctx, serviceMethod, args, reply)
				if err == nil {
					return nil
				}

			}
			s.clients.Delete(provider.ProviderKey)
			if client != nil {
				client.Close()
			}
			client, _ = s.getClient(provider) // 同一台机器
		}

		return errors.New("conn failed")
	case FailOver:
		for retrys > 0 {
			retrys--
			if client != nil {
				err := WrapperCall(s, client.Call)(ctx, serviceMethod, args, reply)
				if err == nil {
					return nil
				}

			}
			s.clients.Delete(provider.ProviderKey)
			if client != nil {
				client.Close()
			}
			provider, client, err = s.SelectClient(ctx, serviceMethod, args) // 重新选择机器

		}

		return errors.New("conn failed")

	default:
		if client != nil {
			err := WrapperCall(s, client.Call)(ctx, serviceMethod, args, reply)
			if err != nil {
				return err
			}
		}

		return nil
	}
}

func WrapperCall(s *sgClient, f CallFunc) CallFunc {
	for _, wrapper := range s.option.Wrappers {
		f = wrapper.WrapperCall(s, f)
	}
	return f
}

// 按照均衡算法，选择机器创建链接
func (s *sgClient) SelectClient(ctx context.Context, serviceMethod string, args interface{}) (registry.Provider, RPCClient, error) {
	provider, err := s.option.Selector.Next(ctx, s.servers, serviceMethod, args, s.option.SelectOption)
	if err != nil {
		return provider, nil, err
	}
	client, err := s.getClient(provider)
	if err != nil {
		return provider, client, err
	}
	return provider, client, nil
}

// 获取链接
func (s *sgClient) getClient(provider registry.Provider) (RPCClient, error) {

	var client RPCClient
	var err error

	// 先从缓存中拿链接
	clientInterface, ok := s.clients.Load(provider.ProviderKey)
	if ok {
		client = clientInterface.(RPCClient)
		if client.IsShutDown() {
			s.clients.Delete(provider.ProviderKey)
			client.Close()
		} else {
			return client, nil
		}
	}

	// 缓存中拿不到，这里再拿一次，防止其他请求已经写入
	clientInterface, ok = s.clients.Load(provider.ProviderKey)
	if !ok {
		// 缓存中没有，则直接创建一个
		client, err = NewSimpleClient(provider.Network, provider.Addr, s.option.Option)
		if err != nil {
			return client, err
		}
		s.clients.Store(provider.ProviderKey, client)
	} else {
		client = clientInterface.(RPCClient)
	}

	return client, nil
}

func NewSGClient(option SGOption) SGClient {

	sgClient := new(sgClient)
	sgClient.option = option

	providers := sgClient.option.Registry.GetServiceList()
	watcher := sgClient.option.Registry.Watch()
	go sgClient.watchService(watcher)
	sgClient.mutex.Lock()
	defer sgClient.mutex.Unlock()

	for _, provider := range providers {
		sgClient.servers = append(sgClient.servers, provider)
	}
	return sgClient
}

// 监听器监听事件， 维护更新providers列表
func (s *sgClient) watchService(watcher registry.Watcher) {

	if watcher == nil {
		fmt.Println("rpc client: watcher is nil")
		return
	}

	for {

		event, err := watcher.Next()
		if err != nil {
			fmt.Printf("rpc client: watcher.Next err. err=%v\n", err)
			continue
		}

		if event.AppKey == s.option.AppKey {
			switch event.Action {
			case registry.Create:
				s.mutex.Lock()
				for _, provider := range event.Providers {
					var isExist bool
					for _, p := range s.servers {
						if provider.ProviderKey == p.ProviderKey {
							isExist = true
							break
						}
					}
					if !isExist {
						s.servers = append(s.servers, provider)
					}
				}
				s.mutex.Unlock()
			case registry.Update:
				s.mutex.Lock()
				for _, provider := range event.Providers {
					for i, p := range s.servers {
						if provider.ProviderKey == p.ProviderKey {
							s.servers[i] = provider
						}
					}
				}
				s.mutex.Unlock()

			case registry.Delete:
				s.mutex.Lock()
				var newServers []registry.Provider
				for _, provider := range event.Providers {
					for _, p := range s.servers {
						if provider.ProviderKey != p.ProviderKey {
							newServers = append(newServers, provider)
						}
					}
				}
				s.servers = newServers
				s.mutex.Unlock()
			}
		}
	}
}
