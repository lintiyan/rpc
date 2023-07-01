package server

import (
	"distribute/rpc/codec"
	"distribute/rpc/protocol"
	"distribute/rpc/transport"
)

type Option struct {
	ProtocolType  protocol.ProtocolType
	SerializeType codec.SerializeType
	CompressType  protocol.CompressType
	TransportType transport.TransportType
}

var DefaultOption = Option{
	ProtocolType: protocol.DefaultProtocolType,
	SerializeType: codec.JsonSerializeType,
	TransportType: transport.TCPTransportType,
}