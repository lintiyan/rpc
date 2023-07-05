package registry

type Registry interface {
	Register(RegisterOption, ...Provider)
	UnRegister(RegisterOption, ...Provider)
	GetServiceList() []Provider
	Watch() Watcher
	UnWatch(watcher Watcher)
}

type RegisterOption struct {
	AppKey string // 服务标识
}

// 服务提供者，服务的某个实例
type Provider struct {
	ProviderKey string // network + @ + addr
	Network     string
	Addr        string
	Meta        map[string]string // 元数据，包含该实例提供的服务，及各服务的方法列表
}

type EventAction byte // 事件类型

const (
	Create EventAction = iota
	Update
	Delete
)

type Event struct {
	Action    EventAction // 事件类型
	AppKey    string      // 是哪个服务
	Providers []Provider  // 变更的服务实例
}

// 监听器，监听事件的发生
type Watcher interface {
	Next() (*Event, error) // 获取事件
	Close()
}
