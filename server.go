package nami

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"reflect"
	"strings"
	"sync"

	"github.com/xeasy/nami/codec"
)

type request struct {
	h            *codec.Header
	svc          *service
	mtype        *methodType
	argv, replyv reflect.Value
}

type NServer interface {
	Accept(lis net.Listener)
	ServeConn(conn io.ReadWriteCloser)
	Regiest(rcvr any) error
	findService(serviceMethod string) (svc *service, mtype *methodType)
	serveCodec(cc codec.Codec)
	readRequestHeader(cc codec.Codec) (h *codec.Header, err error)
	readRequest(cc codec.Codec) (*request, error)
	handleRequest(cc codec.Codec, req *request, sending *sync.Mutex, wg *sync.WaitGroup)
	sendResponse(cc codec.Codec, h *codec.Header, body any, sending *sync.Mutex)
}

// Server represents an RPC Server.
type Server struct {
	serviceMap sync.Map
}

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

func (s *Server) Regiest(rcvr any) error {
	service := newService(rcvr)
	if _, dup := s.serviceMap.LoadOrStore(service.name, service); dup {
		return errors.New("rpc server: service regiest fail, already defined: " + service.name)
	}
	return nil
}

func Regiest(rcvr any) error {
	return DefaultServer.Regiest(rcvr)
}

func (s *Server) findService(serviceMethod string) (svc *service, mtype *methodType, err error) {
	dot := strings.LastIndex(serviceMethod, ".")
	if dot < 0 {
		err = errors.New("rpc server: service/method request ill-formed: " + serviceMethod)
		return
	}
	serviceName, methodName := serviceMethod[:dot], serviceMethod[dot+1:]
	svcIntface, ok := s.serviceMap.Load(serviceName)
	if !ok {
		err = errors.New("rpc server: can't find service " + serviceName)
		return
	}

	svc = svcIntface.(*service)
	mtype = svc.method[methodName]
	if mtype == nil {
		err = errors.New("rpc server: can't find method " + methodName)
		return
	}
	return
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
	req.svc, req.mtype, err = s.findService(h.ServiceMethod)
	if err != nil {
		return req, err
	}
	req.argv = req.mtype.newArgv()
	req.replyv = req.mtype.newReplyv()

	// make sure that argvi is a pointer, Readbody need a pointer as parameter
	argvi := req.argv.Interface()
	if req.argv.Type().Kind() != reflect.Ptr {
		argvi = req.argv.Addr().Interface()
	}
	if err = cc.ReadBody(argvi); err != nil {
		fmt.Println("rcp server: read body fail: ", err)
		return req, err
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

	err := req.svc.call(req.mtype, req.argv, req.replyv)
	if err != nil {
		req.h.Error = err.Error()
		s.sendResponse(cc, req.h, invalidRequest, sending)
		return
	}
	s.sendResponse(cc, req.h, req.replyv.Interface(), sending)
}

func (s *Server) sendResponse(cc codec.Codec, h *codec.Header, body any, sending *sync.Mutex) {
	sending.Lock()
	defer sending.Unlock()

	if err := cc.Write(h, body); err != nil {
		fmt.Println("rpc server: write response error: ", err)
	}
}
