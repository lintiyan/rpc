package memory

import (
	"distribute/rpc/registry"
	"errors"
	"github.com/hashicorp/go-uuid"
	"sync"
)

type MemRegistry struct {
	mu        sync.RWMutex
	Providers []registry.Provider
	watchers  map[string]Watcher
}

func (p *MemRegistry) Register(registerOption registry.RegisterOption, providers ...registry.Provider) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 发送注册事件
	p.sendWatcherEvent(registry.Create, registerOption.AppKey, providers...)

	var newProviders []registry.Provider
	for _, provider := range providers {
		var isExist bool
		for _, p := range p.Providers {
			if provider.ProviderKey == p.ProviderKey {
				isExist = true
				break
			}
		}
		if !isExist {
			newProviders = append(newProviders, provider)
		}
	}

	p.Providers = append(p.Providers, newProviders...)
}

func (p *MemRegistry) UnRegister(registerOption registry.RegisterOption, providers ...registry.Provider) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 发送注册事件
	p.sendWatcherEvent(registry.Delete, registerOption.AppKey, providers...)

	var newProviders []registry.Provider
	for _, provider := range providers {
		for _, p := range p.Providers {
			if provider.ProviderKey != p.ProviderKey {
				newProviders = append(newProviders, p)
			}
		}
	}

	p.Providers = newProviders

}

func (p *MemRegistry) GetServiceList() []registry.Provider {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.Providers
}

func (p *MemRegistry) Watch() registry.Watcher {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.watchers == nil {
		p.watchers = make(map[string]Watcher)
	}

	id, _ := uuid.GenerateUUID()
	eventChan := make(chan *registry.Event)
	exitChan := make(chan bool)
	watcher := Watcher{
		id:   id,
		res:  eventChan,
		exit: exitChan,
	}

	p.watchers[id] = watcher

	return &watcher
}

func (p *MemRegistry) UnWatch(watcher registry.Watcher) {

	target, ok := watcher.(*Watcher)
	if !ok {
		return
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	for id := range p.watchers {
		if id == target.id {
			delete(p.watchers, id)
		}
	}

	return
}

func (p *MemRegistry) sendWatcherEvent(action registry.EventAction, appKey string, providers ...registry.Provider) {

	event := &registry.Event{
		Action:    action,
		AppKey:    appKey,
		Providers: providers,
	}

	var watchers []Watcher
	p.mu.RLock()
	for _, watcher := range p.watchers {
		watchers = append(watchers, watcher)
	}
	p.mu.RUnlock()

	for _, watcher := range watchers {
		select {
		case <-watcher.exit:
			p.mu.RLock()
			delete(p.watchers, watcher.id)
			p.mu.RUnlock()
			continue
		default:
			watcher.res <- event
		}
	}
}

type Watcher struct {
	id   string
	res  chan *registry.Event
	exit chan bool
}

func (w *Watcher) Next() (*registry.Event, error) {
	select {
	case event := <-w.res:
		return event, nil
	case <-w.exit:
		return nil, errors.New("watcher is exit")
	}
}

func (w *Watcher) Close() {
	select {
	case <-w.exit: // 已经关闭
		return
	default:
		close(w.exit)
	}
}
