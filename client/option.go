package client

import (
	"distribute/rpc/codec"
	"distribute/rpc/protocol"
	"distribute/rpc/registry"
	"distribute/rpc/registry/p2p"
	"distribute/rpc/selector"
	"distribute/rpc/transport"
	"time"
)

type Option struct {
	ProtocolType  protocol.ProtocolType   // 网络通信协议类型
	SerializeType codec.SerializeType     // data数据序列化类型
	CompressType  protocol.CompressType   // 数据压缩类型，貌似没用到todo
	TransportType transport.TransportType // 网络通信类型，目前用的是tcp

	RequestTimeOut time.Duration // 接口请求的过期时间，针对所有接口。 后续实现针对单个接口单独配置过期时间
}

var DefaultOption = Option{
	ProtocolType:   protocol.DefaultProtocolType,
	SerializeType:  codec.JsonSerializeType,
	TransportType:  transport.TCPTransportType,
	RequestTimeOut: 10 * time.Second,
}

type FailMode byte

const (
	FailFast  = iota // 快速失败，有异常直接抛出
	FailOver         // 异常时，重新生成链接重试， 会链接到其他机器
	FailRetry        // 异常时，使用原来的链接重试
	FailSafe         //  忽略失败，直接返回
)

type SGOption struct {
	Option

	Selector     selector.Selector     // 负载均衡策略
	SelectOption selector.SelectOption // 负载均衡选项，可自定义过滤主机策略

	Wrappers []Wrapper // 增强器

	FailMode FailMode // 请求失败策略
	Retries  int      // 失败重试次数

	Registry registry.Registry // 服务注册发现策略

	AppKey string // 该客户端所要链接的服务的key
}

var DefaultSGOption = SGOption{
	Option:   DefaultOption,
	Selector: &selector.RandomSelector{},
	Wrappers: []Wrapper{&LogWrapper{}},
	FailMode: FailOver,
	Retries:  1,
	Registry: &p2p.PTPRegistry{},

	AppKey: "",
}
