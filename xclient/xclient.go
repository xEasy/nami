package xclient

import (
	"context"
	"io"
	"reflect"
	"sync"

	"github.com/xeasy/nami"
	"github.com/xeasy/nami/client"
)

type XClient struct {
	d       Discovery
	mode    SelectMode
	opt     *nami.Option
	mu      sync.Mutex
	clients map[string]client.NClient // client cache
}

// make sure XClient represented io.Closer interface
var _ io.Closer = (*XClient)(nil)

func NewXClient(d Discovery, mode SelectMode, opt *nami.Option) *XClient {
	return &XClient{
		d:       d,
		mode:    mode,
		opt:     opt,
		clients: make(map[string]client.NClient),
	}
}

func (x *XClient) Close() error {
	x.mu.Lock()
	defer x.mu.Unlock()

	for key, client := range x.clients {
		client.Close()
		delete(x.clients, key)
	}
	return nil
}

// dial get a cache client, make a new client if cached client unavailable
func (xc *XClient) dial(rpcAddr string) (client.NClient, error) {
	xc.mu.Lock()
	defer xc.mu.Unlock()

	cli, ok := xc.clients[rpcAddr]
	if ok && !cli.IsAvailable() {
		cli.Close()
		delete(xc.clients, rpcAddr)
		cli = nil
	}

	if cli == nil {
		var err error
		cli, err = client.XDial(rpcAddr, xc.opt)
		if err != nil {
			return nil, err
		}
		xc.clients[rpcAddr] = cli
	}
	return cli, nil
}

func (xc *XClient) call(rpcAddr string, ctx context.Context, serviceMethod string, args, reply any) error {
	cli, err := xc.dial(rpcAddr)
	if err != nil {
		return err
	}
	return cli.Call(ctx, serviceMethod, args, reply)
}

func (xc *XClient) Call(ctx context.Context, serviceMethod string, args, reply any) error {
	rpcAddr, err := xc.d.Get(xc.mode)
	if err != nil {
		return err
	}
	return xc.call(rpcAddr, ctx, serviceMethod, args, reply)
}

func (xc *XClient) Broadcast(ctx context.Context, serviceMethod string, args, reply any) error {
	servers, err := xc.d.GetAll()
	if err != nil {
		return err
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	var e error
	replyDone := reply == nil // if reply is nil, don't need to set value
	ctx, cancel := context.WithCancel(ctx)
	for _, rpcAddr := range servers {
		wg.Add(1)
		go func(rpcAddr string) {
			defer wg.Done()
			var cloneReply any
			if reply != nil {
				cloneReply = reflect.New(reflect.ValueOf(reply).Elem().Type()).Interface()
			}
			err := xc.call(rpcAddr, ctx, serviceMethod, args, cloneReply)
			if err != nil && e != nil {
				e = err
				cancel()
			}
			if err == nil && !replyDone {
				reflect.ValueOf(reply).Elem().Set(reflect.ValueOf(cloneReply).Elem())
				replyDone = true
			}
			mu.Unlock()
		}(rpcAddr)
	}
	wg.Wait()
	// avoid context leak
	cancel()
	return e
}
