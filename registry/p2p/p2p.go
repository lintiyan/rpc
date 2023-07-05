package p2p

import "distribute/rpc/registry"

type PTPRegistry struct {
	Providers []registry.Provider
}


func (p *PTPRegistry) Register(registerOption registry.RegisterOption, providers ...registry.Provider) {
	p.Providers = append(p.Providers, providers...)
}

func (p *PTPRegistry) UnRegister(registerOption registry.RegisterOption, providers ...registry.Provider) {
	p.Providers = []registry.Provider{}
}

func (p *PTPRegistry) GetServiceList() []registry.Provider {
	return p.Providers
}

func (p *PTPRegistry) Watch() registry.Watcher {
	return nil
}

func (p *PTPRegistry) UnWatch(watcher registry.Watcher) {
	return
}
