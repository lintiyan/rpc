package server

import (
	"distribute/rpc/codec"
	"distribute/rpc/protocol"
	"distribute/rpc/registry"
	"distribute/rpc/transport"
)

type Option struct {
	ProtocolType   protocol.ProtocolType		// 网络通信协议类型
	SerializeType  codec.SerializeType			// data数据序列化方式
	CompressType   protocol.CompressType		// 压缩格式类型
	TransportType  transport.TransportType		// 网络通信类型，tcp
	Wrappers       []Wrapper					// 增强器，对接口功能进行加强
	Registry       registry.Registry			// 服务注册发现策略
	RegistryOption registry.RegisterOption		// 服务注册事件中配置的服务标识， 用于客户端判断事件是否需要关注。客户端只需关注它需要关注的服务的事件
	ShutdownWait   int							// 关机等待时间，优雅推出

	AppKey string								// 本服务的appkey
}

var DefaultOption = Option{
	ProtocolType:  protocol.DefaultProtocolType,
	SerializeType: codec.JsonSerializeType,
	TransportType: transport.TCPTransportType,
}
