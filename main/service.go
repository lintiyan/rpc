package main

import (
	"context"
	"fmt"
)

type MathService struct{}

type Args struct {
	A int
	B int
}

type Reply struct {
	C int
}

func (m MathService) Add(ctx context.Context, args *Args, reply *Reply) error {
	fmt.Println("recv req. a=", args.A, " b=", args.B)
	reply.C = args.A + args.B
	return nil
}

func (m MathService) Minus(ctx context.Context, args *Args, reply *Reply) error {
	reply.C = args.A - args.B
	return nil
}
