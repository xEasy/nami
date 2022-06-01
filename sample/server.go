package main

import (
	"net"
	"net/http"

	"github.com/xeasy/nami"
)

type Foo int

type Args struct {
	Num1, Num2 int
}

func (f Foo) Sum(args Args, reply *int) error {
	*reply = args.Num1 + args.Num2
	return nil
}

func main() {
	startServer()
}

func startServer() {
	var foo Foo
	if err := nami.Regiest(&foo); err != nil {
		panic("regist error:" + err.Error())
	}

	l, err := net.Listen("tcp", "127.0.0.1:8999")
	if err != nil {
		panic(err)
	}
	nami.HandleHTTP()
	http.Serve(l, nil)
}
