package main

import (
	"context"
	"distribute/rpc/client"
	"distribute/rpc/registry/memory"
	"distribute/rpc/server"
	"fmt"
	"time"
)

func main() {

	serviceOption := server.DefaultOption

	//registry := &p2p.PTPRegistry{}
	registry := &memory.MemRegistry{}
	serviceOption.Registry = registry
	serviceOption.RegistryOption.AppKey = "my-app"
	serviceOption.Wrappers = append(serviceOption.Wrappers, &server.DefaultWrapper{})
	serviceOption.ShutdownWait = 6
	srv := server.NewSGServer(serviceOption)

	metaData := make(map[string]string)
	err := srv.Register(MathService{}, metaData)
	if err != nil {
		fmt.Println(err)
		return
	}

	go func() {
		err := srv.Serve("tcp", ":9999")
		if err != nil {
			panic(err)
		}
	}()

	go func() {
		err := srv.Serve("tcp", ":9998")
		if err != nil {
			panic(err)
		}
	}()

	time.Sleep(5 * time.Second)

	fmt.Println("providers= ",registry.Providers)

	clientOption := client.DefaultSGOption
	clientOption.AppKey = "my-app"
	clientOption.Registry = registry
	cli := client.NewSGClient(clientOption)

	ctx := context.Background()

	//args := Args{1, 2}
	//reply := Reply{}
	//err = cli.Call(ctx, "MathService_Add", &args, &reply)
	//if err != nil {
	//	fmt.Println(err)
	//	return
	//}
	//fmt.Println("add result = ", reply.C)

	args2 := &Args{3, 5}
	reply2 := &Reply{}
	var done = make(chan *client.Call, 1)
	cli.Go(ctx, "MathService_Add", args2, reply2, done)
	go func() {
		select {
		case call := <-done :
			fmt.Println(call.Reply)
		}
	}()

	time.Sleep(5 * time.Second)

	srv.Close()
}
