package nami

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/xeasy/nami/codec"
)

type request struct {
	h            *codec.Header
	argv, replyv reflect.Value
}

type NServer interface {
	Accept(lis net.Listener)
	ServeConn(conn io.ReadWriteCloser)
	serveCodec(cc codec.Codec)
	readRequestHeader(cc codec.Codec) (h *codec.Header, err error)
	readRequest(cc codec.Codec) (*request, error)
	handleRequest(cc codec.Codec, req *request, sending *sync.Mutex, wg *sync.WaitGroup)
	sendResponse(cc codec.Codec, h *codec.Header, body any, sending *sync.Mutex)
}

// Server represents an RPC Server.
type Server struct{}

var DefaultServer *Server
var invalidRequest = struct{}{}

func init() {
	DefaultServer = NewServer()
}

// NewServer return a new Server.
func NewServer() *Server {
	return &Server{}
}

func Accept(l net.Listener) {
	DefaultServer.Accept(l)
}

func (s *Server) Accept(lis net.Listener) {
	for {
		conn, err := lis.Accept()
		if err != nil {
			fmt.Println("rpc server: accept error: ", err)
			return
		}
		s.ServeConn(conn)
	}
}
func (s *Server) ServeConn(conn io.ReadWriteCloser) {
	defer func() { conn.Close() }()

	var opt Option
	if err := json.NewDecoder(conn).Decode(&opt); err != nil {
		fmt.Println("rcp server: options error: ", err)
		return
	}

	if opt.MagicNumber != MagicNumber {
		fmt.Println("rpc server: invalid MagicNumber :", opt.MagicNumber)
		return
	}

	codecFunc := codec.NewCodecFuncMap[opt.CodecType]
	if codecFunc == nil {
		fmt.Println("rpc server: invalid codec type : ", opt.CodecType)
		return
	}
	s.serveCodec(codecFunc(conn))
}

func (s *Server) serveCodec(cc codec.Codec) {
	var sending sync.Mutex
	var wg sync.WaitGroup
	for {
		req, err := s.readRequest(cc)
		if err != nil {
			fmt.Println("rpc server: read request error: ", err)
			if req == nil {
				break
			}
			req.h.Error = err.Error()
			s.sendResponse(cc, req.h, invalidRequest, &sending)
		}
		wg.Add(1)
		go s.handleRequest(cc, req, &sending, &wg)
	}
}

func (s *Server) readRequest(cc codec.Codec) (*request, error) {
	h, err := s.readRequestHeader(cc)
	if err != nil {
		return nil, err
	}
	req := &request{h: h}
	req.argv = reflect.New(reflect.TypeOf(""))
	if err = cc.ReadBody(req.argv.Interface()); err != nil {
		fmt.Println("rpc server: read argv err: ", err)
	}
	return req, nil
}

func (s *Server) readRequestHeader(cc codec.Codec) (*codec.Header, error) {
	var h codec.Header
	if err := cc.ReadHeader(&h); err != nil {
		if err != io.EOF && err != io.ErrUnexpectedEOF {
			fmt.Println("rpc server: read header error: ", err)
		}
		return nil, err
	}
	return &h, nil
}

func (s *Server) handleRequest(cc codec.Codec, req *request, sending *sync.Mutex, wg *sync.WaitGroup) {
	defer wg.Done()

	fmt.Println(req.h, req.argv.Elem())
	req.replyv = reflect.ValueOf(strings.Join([]string{"rpc resp", strconv.Itoa(int(req.h.Seq))}, ""))
	s.sendResponse(cc, req.h, req.replyv.Interface(), sending)
}

func (s *Server) sendResponse(cc codec.Codec, h *codec.Header, body any, sending *sync.Mutex) {
	sending.Lock()
	defer sending.Unlock()

	if err := cc.Write(h, body); err != nil {
		fmt.Println("rpc server: write response error: ", err)
	}
}
