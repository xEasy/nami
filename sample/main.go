package main

import (
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/xeasy/nami"
	"github.com/xeasy/nami/codec"
)

func main() {
	addr := make(chan string)
	go startServer(addr)

	conn, _ := net.Dial("tcp", <-addr)

	defer func() { conn.Close() }()

	time.Sleep(time.Second)

	// send options
	json.NewEncoder(conn).Encode(nami.DefaultOption)
	cc := codec.NewGobCodec(conn)

	// send request & recive
	for i := 0; i < 5; i++ {
		h := &codec.Header{
			ServiceMethod: "Go.South",
			Seq:           uint64(i),
		}
		cc.Write(h, fmt.Sprintf("rpc req %d", h.Seq))
		cc.ReadHeader(h)
		var reply string
		cc.ReadBody(&reply)
		fmt.Println("reply:", reply)
	}
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
