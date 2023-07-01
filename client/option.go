package client

import (
	"distribute/rpc/codec"
	"distribute/rpc/protocol"
	"distribute/rpc/transport"
	"time"
)

type Option struct {
	ProtocolType  protocol.ProtocolType
	SerializeType codec.SerializeType
	CompressType  protocol.CompressType
	TransportType transport.TransportType

	RequestTimeOut time.Duration
}

var DefaultOption = Option{
	ProtocolType:   protocol.DefaultProtocolType,
	SerializeType:  codec.JsonSerializeType,
	TransportType:  transport.TCPTransportType,
	RequestTimeOut: 10 * time.Second,
}
