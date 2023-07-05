package client

import (
	"context"
	"fmt"
)

type GoFunc func(ctx context.Context, serviceMethod string, args interface{}, reply interface{}, done chan *Call) (*Call, error)
type CallFunc func(ctx context.Context, serviceMethod string, args interface{}, reply interface{}) error

type Wrapper interface {
	WrapperGo(s *sgClient, goFunc GoFunc) GoFunc
	WrapperCall(s *sgClient, callFunc CallFunc) CallFunc
}

type LogWrapper struct{}

func (l *LogWrapper) WrapperGo(s *sgClient, goFunc GoFunc) GoFunc {
	return func(ctx context.Context, serviceMethod string, args interface{}, reply interface{}, done chan *Call) (*Call, error) {
		fmt.Println(fmt.Sprintf("before Go. serviceMethod=%s args=%v reply=%v", serviceMethod, args, reply))
		call, err := goFunc(ctx, serviceMethod, args, reply, done)
		fmt.Println(fmt.Sprintf("after Go. serviceMethod=%s args=%v reply=%v", serviceMethod, args, reply))
		return call, err
	}
}

func (l *LogWrapper) WrapperCall(s *sgClient, callFunc CallFunc) CallFunc {
	return func(ctx context.Context, serviceMethod string, args interface{}, reply interface{}) error {
		fmt.Println(fmt.Sprintf("before Call. serviceMethod=%s args=%v reply=%v", serviceMethod, args, reply))
		err := callFunc(ctx, serviceMethod, args, reply)
		fmt.Println(fmt.Sprintf("after Call. serviceMethod=%s args=%v reply=%v", serviceMethod, args, reply))
		return err
	}
}

