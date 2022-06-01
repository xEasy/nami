package main

import (
	"context"
	"fmt"
	"sync"

	"github.com/xeasy/nami/client"
)

func main() {
	client, _ := client.DialHTTP("tcp", "127.0.0.1:8999")
	defer func() { client.Close() }()

	// send & receive
	var wg sync.WaitGroup

	for i := 0; i < 5; i++ {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()
			var reply int
			ctx := context.Background()
			if err := client.Call(ctx, "Foo.Sum", &struct{ Num1, Num2 int }{i, i * i}, &reply); err != nil {
				fmt.Println("call Go.south fail: ", err)
				return
			}
			fmt.Printf("%d + %d = %d \n", i, i*i, reply)
		}(i)
	}

	wg.Wait()
}
