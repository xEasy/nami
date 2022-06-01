package client

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/xeasy/nami"
	"github.com/xeasy/nami/codec"
)

type newClientFunc func(conn net.Conn, opt *nami.Option) (NClient, error)

type clientResult struct {
	client NClient
	err    error
}

type NClient interface {
	io.Closer
	receive()
	send(call *Call)
	registerCall(call *Call) (uint64, error)
	removeCall(seq uint64) *Call
	terminateCalls(err error)
	Go(serviceMethod string, args, reply any, done chan *Call) *Call
	Call(ctx context.Context, serviceMethod string, args, reply any) error
	IsAvailable() bool
}

type Client struct {
	cc       codec.Codec
	opt      *nami.Option
	sending  sync.Mutex
	header   codec.Header
	mu       sync.Mutex
	seq      uint64
	pending  map[uint64]*Call
	closing  bool
	shutdown bool
}

var ErrShutdown = errors.New("connection is shut down")

func NewHTTPClient(conn net.Conn, opt *nami.Option) (NClient, error) {
	_, _ = io.WriteString(conn, fmt.Sprintf("CONNECT %s HTTP/1.0\n\n", nami.DefaultRPCPath))

	resp, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: "CONNECT"})
	fmt.Println("resp", err)
	if err != nil {
		err = errors.New("unexpected HTTP response: " + err.Error())
	}
	if err == nil || resp.Status == nami.Connected {
		return NewClient(conn, opt)
	}
	if err == nil {
		err = errors.New("unexpected HTTP response: " + err.Error())
	}
	return nil, err
}

func NewClient(conn net.Conn, opt *nami.Option) (NClient, error) {
	f := codec.NewCodecFuncMap[opt.CodecType]
	if f == nil {
		err := fmt.Errorf("invalid codec type %s", opt.CodecType)
		fmt.Println("rpc client: codec error: ", err)
		return nil, err
	}

	// send options with server
	if err := json.NewEncoder(conn).Encode(opt); err != nil {
		fmt.Println("rpc client: option error:", err)
		conn.Close()
		return nil, err
	}

	return newClientcodec(f(conn), opt), nil
}

func newClientcodec(cc codec.Codec, opt *nami.Option) NClient {
	client := &Client{
		seq:      1,
		cc:       cc,
		opt:      opt,
		closing:  false,
		shutdown: false,
		pending:  make(map[uint64]*Call),
	}

	go client.receive()
	return client
}

func parseOptions(opts ...*nami.Option) (*nami.Option, error) {
	if len(opts) == 0 || opts[0] == nil {
		return nami.DefaultOption, nil
	}

	if len(opts) != 1 {
		return nil, errors.New("too many options, more than 1")
	}
	opt := opts[0]
	opt.MagicNumber = nami.DefaultOption.MagicNumber
	if opt.CodecType == "" {
		opt.CodecType = nami.DefaultOption.CodecType
	}

	return opt, nil
}

// rpcAddr eg, http@10.0.0.1:7001, tcp@10.0.0.1:9999, unix@/tmp/geerpc.sock
func XDial(rpcAddr string, opts ...*nami.Option) (NClient, error) {
	parts := strings.Split(rpcAddr, "@")
	if len(parts) != 2 {
		return nil, fmt.Errorf("rpc client: wrong addr format %s, expect protool@address", rpcAddr)
	}
	protool, addr := parts[0], parts[1]
	switch protool {
	case "http":
		return DialHTTP("tcp", addr, opts...)
	default:
		return Dial(protool, addr, opts...)
	}
}

func Dial(nw, addr string, opts ...*nami.Option) (NClient, error) {
	return dialTimeout(NewClient, nw, addr, opts...)
}

func DialHTTP(nw, addr string, opts ...*nami.Option) (NClient, error) {
	return dialTimeout(NewHTTPClient, nw, addr, opts...)
}

func dialTimeout(ncFunc newClientFunc, nw, addr string, opts ...*nami.Option) (NClient, error) {
	opt, err := parseOptions(opts...)
	if err != nil {
		return nil, err
	}
	conn, err := net.DialTimeout(nw, addr, opt.ConnectionTimeout)
	if err != nil {
		return nil, err
	}

	ch := make(chan clientResult)
	go func() {
		client, err := ncFunc(conn, opt)
		if client == nil {
			conn.Close()
		}
		ch <- clientResult{client, err}
	}()

	if opt.ConnectionTimeout == 0 {
		result := <-ch
		return result.client, result.err
	}

	select {
	case <-time.After(opt.ConnectionTimeout):
		return nil, fmt.Errorf("rpc client: connect timeout: expect within %s", opt.ConnectionTimeout)
	case result := <-ch:
		return result.client, result.err
	}
}

func (client *Client) Close() error {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.closing {
		return ErrShutdown
	}
	client.closing = true
	return client.cc.Close()
}

func (client *Client) IsAvailable() bool {
	client.mu.Lock()
	defer client.mu.Unlock()
	return !client.shutdown && !client.closing
}

func (c *Client) receive() {
	var err error
	for err == nil {
		var h codec.Header
		if err := c.cc.ReadHeader(&h); err != nil {
			break
		}

		call := c.removeCall(h.Seq)
		switch {
		case call == nil:
			// it means write failed and call was removed
			err = c.cc.ReadBody(nil)
		case h.Error != "":
			call.Error = fmt.Errorf(h.Error)
			err = c.cc.ReadBody(nil)
			call.done()
		default:
			err = c.cc.ReadBody(call.Reply)
			if err != nil {
				call.Error = errors.New("reading body fail: " + err.Error())
			}
			call.done()
		}
	}
	c.terminateCalls(err)
}

func (c *Client) registerCall(call *Call) (uint64, error) {
	// IsAvailable acquired mu lock
	if !c.IsAvailable() {
		return 0, ErrShutdown
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	call.Seq = c.seq
	c.pending[call.Seq] = call
	c.seq++
	return call.Seq, nil
}

func (c *Client) removeCall(seq uint64) *Call {
	c.mu.Lock()
	defer c.mu.Unlock()
	call := c.pending[seq]
	delete(c.pending, seq)
	return call
}

func (c *Client) terminateCalls(err error) {
	c.sending.Lock()
	defer c.sending.Unlock()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.shutdown = true
	for _, call := range c.pending {
		call.Error = err
		call.done()
	}
}

func (c *Client) send(call *Call) {
	// make sure client will send complete request
	c.sending.Lock()
	defer c.sending.Unlock()

	// regiest call
	seq, err := c.registerCall(call)
	if err != nil {
		call.Error = err
		call.done()
		return
	}

	// prepare request header
	c.header.ServiceMethod = call.ServiceMethod
	c.header.Seq = call.Seq
	c.header.Error = ""

	// ecode and send the request
	if err := c.cc.Write(&c.header, call.Args); err != nil {
		call := c.removeCall(seq)

		// call may be nil, it means write failed, client has received the response and handled
		if call != nil {
			call.Error = err
			call.done()
		}
	}
}

func (c *Client) Go(serviceMethod string, args any, reply any, done chan *Call) *Call {
	if done == nil {
		done = make(chan *Call, 1)
	} else if cap(done) == 0 {
		panic("rpc client: done channel is unbuffed")
	}

	call := &Call{
		ServiceMethod: serviceMethod,
		Args:          args,
		Reply:         reply,
		Done:          done,
	}
	c.send(call)
	return call
}

func (c *Client) Call(ctx context.Context, serviceMethod string, args any, reply any) error {
	call := c.Go(serviceMethod, args, reply, make(chan *Call, 1))
	select {
	case <-ctx.Done():
		c.removeCall(call.Seq)
		return errors.New("rpc client: call failed: " + ctx.Err().Error())
	case call := <-call.Done:
		return call.Error
	}
}
