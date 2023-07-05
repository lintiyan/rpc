package selector

import (
	"context"
	"distribute/rpc/registry"
	"errors"
	"math/rand"
	"time"
)

type Filter func(ctx context.Context, provider registry.Provider, serviceMethod string, args interface{}) bool

type SelectOption struct {
	Filters []Filter
}

type Selector interface {
	Next(ctx context.Context, providers []registry.Provider, serviceMethod string, args interface{}, option SelectOption) (registry.Provider, error)
}

func combineFilter(filters []Filter) Filter {

	return func(ctx context.Context, provider registry.Provider, serviceMethod string, args interface{}) bool {
		for _, f := range filters {
			if !f(ctx, provider, serviceMethod, args) {
				return false
			}
		}
		return true
	}
}

// 随机算法
type RandomSelector struct {
}

func (r *RandomSelector) Next(ctx context.Context, providers []registry.Provider, serviceMethod string, args interface{}, option SelectOption) (registry.Provider, error) {
	filter := combineFilter(option.Filters)

	var newProviders []registry.Provider
	for _, provider := range providers {
		if filter(ctx, provider, serviceMethod, args) {
			newProviders = append(newProviders, provider)
		}
	}

	if len(newProviders) == 0 {
		return registry.Provider{}, errors.New("has no valid provider")
	}

	rad := rand.New(rand.NewSource(time.Now().UnixNano()))

	return newProviders[rad.Intn(len(newProviders))], nil
}
