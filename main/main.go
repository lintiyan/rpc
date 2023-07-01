package main

import (
	"context"
	"distribute/rpc/client"
	"distribute/rpc/server"
	"fmt"
	"time"
)

func main() {

	serviceOption := server.DefaultOption
	srv := server.NewSimpleServer(serviceOption)

	metaData := make(map[string]string)
	err := srv.Register(MathService{}, metaData)
	if err != nil {
		fmt.Println(err)
		return
	}

	go func() {
		fmt.Println(time.Now().Format("2006-01-02 15:04:05"))
		err := srv.Serve("tcp", ":9999")
		if err != nil {
			panic(err)
		}
	}()

	time.Sleep(5 * time.Second)

	clientOption := client.DefaultOption
	cli, err := client.NewSimpleClient("tcp", ":9999", clientOption)
	if err != nil {
		panic(err)
	}

	args := Args{1, 2}
	reply := Reply{}
	err = cli.Call(context.Background(), "MathService_Add", &args, &reply)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("add result = ", reply.C)
}
