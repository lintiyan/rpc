package client

import (
	"context"
	"distribute/rpc/codec"
	"distribute/rpc/protocol"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type RPCClient interface {
	Go(ctx context.Context, serviceMethod string, args interface{}, reply interface{}, done chan *Call) error
	Call(ctx context.Context, serviceMethod string, args interface{}, reply interface{}) error
	Close() error
}

type Call struct {
	ServiceMethod string
	Args          interface{}
	Reply         interface{}
	Done          chan *Call // 通道， 作用是判断请求是否结束
	Error         error
}

func (c *Call) done() {
	c.Done <- c
}

type SimpleClient struct {
	rwc          io.ReadWriteCloser
	pendingCalls sync.Map
	mutex        sync.Mutex
	shutdown     bool
	seq          uint64
	Option       Option
}

func NewSimpleClient(network, addr string, option Option) (*SimpleClient, error) {
	sc := &SimpleClient{}

	conn, err := net.Dial(network, addr)
	if err != nil {
		fmt.Println("NewSimpleClient|Dial failed. err=", err)
		return nil, err
	}

	sc.rwc = conn
	sc.Option = option

	go sc.input()

	return sc, nil
}

func (s *SimpleClient) Go(ctx context.Context, serviceMethod string, args interface{}, reply interface{}, done chan *Call) (*Call, error) {

	seq := ctx.Value(protocol.RequestSeqKey).(uint64)
	if seq == 0 {
		ctx = context.WithValue(ctx, protocol.RequestSeqKey, atomic.AddUint64(&s.seq, 1))
	}

	// 发送请求
	if done == nil {
		done = make(chan *Call, 10)
	}

	call := new(Call)
	call.ServiceMethod = serviceMethod
	call.Args = args
	call.Reply = reply
	call.Done = done

	err := s.send(ctx, call)
	if err != nil {
		return nil, err
	}

	return call, err
}

func (s *SimpleClient) Call(ctx context.Context, serviceMethod string, args interface{}, reply interface{}) error {

	seq := atomic.AddUint64(&s.seq, 1)
	ctx = context.WithValue(ctx, protocol.RequestSeqKey, seq)
	var cancelFunc func()
	if s.Option.RequestTimeOut != 0 {
		ctx, cancelFunc = context.WithTimeout(ctx, s.Option.RequestTimeOut)
	}

	done := make(chan *Call, 1)

	call, err := s.Go(ctx, serviceMethod, args, reply, done)
	if err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		s.pendingCalls.Delete(seq)
		cancelFunc()
		fmt.Println(time.Now().Format("2006-01-02 15:04:05"))
		return errors.New("context time exceeded. ")
	case <-call.Done:
		if call.Error != nil {
			return call.Error
		}
		reply = call.Reply
		return nil
	}

}

func (s *SimpleClient) send(ctx context.Context, call *Call) error {

	seq := ctx.Value(protocol.RequestSeqKey).(uint64)
	// 编辑msg
	splits := strings.Split(call.ServiceMethod, "_")
	if len(splits) != 2 {
		return errors.New("service method format error")
	}
	s.pendingCalls.Store(seq, call)
	message := protocol.NewProtocol(s.Option.ProtocolType).NewMessage()
	message.Header.ServiceName = splits[0]
	message.Header.MethodName = splits[1]
	message.Header.SerializeType = s.Option.SerializeType
	message.Header.CompressType = s.Option.CompressType
	message.Header.MessageType = protocol.MessageTypeReq

	message.Seq = seq

	bytes, err := codec.GetCodec(s.Option.SerializeType).Encode(call.Args)
	if err != nil {
		err := errors.New(fmt.Sprintf("send|Encode failed. err=%v", err))
		call.done()
		call.Error = err
		return err
	}

	message.Data = bytes
	encodeMessage := protocol.NewProtocol(s.Option.ProtocolType).EncodeMessage(message)
	_, err = s.rwc.Write(encodeMessage)
	if err != nil {
		err := errors.New(fmt.Sprintf("send|Write failed. err=%v", err))
		call.done()
		call.Error = err

		return err
	}

	return nil
}

func (s *SimpleClient) Close() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.shutdown = true
	s.pendingCalls.Range(func(key, value interface{}) bool {
		call, ok := value.(*Call)
		if ok {
			call.Error = errors.New("client shutdown")
			call.done()
		}
		s.pendingCalls.Delete(key)
		return true
	})
	return nil
}

func (s *SimpleClient) input() {
	// 启动循环，接受请求的响应
	for {
		message, err := protocol.NewProtocol(s.Option.ProtocolType).DecodeMessage(s.rwc)
		if err != nil {
			fmt.Println("input|DecodeMessage failed. err=", err)
			continue
		}

		if message == nil {
			continue
		}

		callInterface, ok := s.pendingCalls.Load(message.Seq)
		if !ok {
			fmt.Println("input|pendingCalls.Load failed. ")
			continue
		}

		call, ok := callInterface.(*Call)
		if !ok {
			fmt.Println("input|callInterface.(*Call) failed. ")
			continue
		}

		if message.StatusCode != protocol.ServiceOKCode || message.Error != "" {
			call.Error = errors.New(message.Error)
			call.done()
			continue
		}

		if call == nil {
			continue
		}

		err = codec.GetCodec(s.Option.SerializeType).Decode(message.Data, call.Reply)
		if err != nil {
			fmt.Println("input|Decode failed. err=", err)
			call.done()
			continue
		}

		call.done()
		s.pendingCalls.Delete(message.Seq)

	}
}
