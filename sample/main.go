package main

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/xeasy/nami"
	"github.com/xeasy/nami/client"
)

func main() {
	addr := make(chan string)
	go startServer(addr)

	client, _ := client.Dial("tcp", <-addr)
	defer func() { client.Close() }()

	time.Sleep(time.Second)

	// send & receive
	var wg sync.WaitGroup

	for i := 0; i < 5; i++ {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()
			args := fmt.Sprintf("rpc req %d", i)
			var reply string
			if err := client.Call("Go.south", args, &reply); err != nil {
				fmt.Println("call Go.south fail: ", err)
				return
			}
			fmt.Println("reply: ", reply)
		}(i)
	}

	wg.Wait()
}

func startServer(addr chan string) {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(err)
	}
	fmt.Println("start rpc server on", l.Addr())
	addr <- l.Addr().String()
	nami.Accept(l)
}
