package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/xeasy/nami"
	"github.com/xeasy/nami/registry"
	"github.com/xeasy/nami/xclient"
)

type Foo int

type Args struct{ Num1, Num2 int }

func (f Foo) Sum(args Args, reply *int) error {
	*reply = args.Num1 + args.Num2
	return nil
}

func (f Foo) Sleep(args Args, reply *int) error {
	time.Sleep(time.Second * time.Duration(args.Num1))
	return nil
}

func startServer(registryAddr string, wg *sync.WaitGroup) {
	var foo Foo
	l, _ := net.Listen("tcp", ":0")
	server := nami.NewServer()
	server.Regiest(&foo)
	registry.Heartbeat(registryAddr, "tcp@"+l.Addr().String(), 0)
	wg.Done()
	server.Accept(l)
}

func foo(xc *xclient.XClient, ctx context.Context, typ, serviceMethod string, args *Args) {
	var reply int
	var err error
	switch typ {
	case "call":
		err = xc.Call(ctx, serviceMethod, args, &reply)
	case "broadcast":
		err = xc.Broadcast(ctx, serviceMethod, args, &reply)
	}
	if err != nil {
		fmt.Printf("%s %s error: %v \n", typ, serviceMethod, err)
	} else {
		fmt.Printf("%s %s success: %d + %d = %d \n", typ, serviceMethod, args.Num1, args.Num2, reply)
	}
}

func call(regiest string) {
	d := xclient.NewRegistryDiscovery(regiest, 0)
	xc := xclient.NewXClient(d, xclient.RandomSelect, nil)
	defer func() {
		xc.Close()
	}()

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			foo(xc, context.Background(), "call", "Foo.Sum", &Args{Num1: i, Num2: i * i})
		}(i)
	}
	wg.Wait()
}

func broadcast(regiestry string) {
	d := xclient.NewRegistryDiscovery(regiestry, 0)
	xc := xclient.NewXClient(d, xclient.RandomSelect, nil)
	defer func() {
		xc.Close()
	}()

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			foo(xc, context.Background(), "call", "Foo.Sum", &Args{Num1: i, Num2: i * i})
			// expect 2 - 5 timeout
			ctx, _ := context.WithTimeout(context.Background(), time.Second*2)
			foo(xc, ctx, "call", "Foo.Sleep", &Args{Num1: i, Num2: i * i})
		}(i)
	}
	wg.Wait()
}

func startRegistry(wg *sync.WaitGroup) {
	l, _ := net.Listen("tcp", ":9999")
	registry.HandlHTTP()
	wg.Done()
	_ = http.Serve(l, nil)
}

func main() {
	regiestryAddr := "http://localhost:9999/_namirpc_/regiestry"

	var wg sync.WaitGroup
	wg.Add(1)
	go startRegistry(&wg)
	wg.Wait()

	wg.Add(2)
	go startServer(regiestryAddr, &wg)
	go startServer(regiestryAddr, &wg)
	wg.Wait()

	time.Sleep(time.Second)
	call(regiestryAddr)
	broadcast(regiestryAddr)
}
