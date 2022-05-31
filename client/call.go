package client

// call represents an active RPC
type Call struct {
	Seq           uint64
	ServiceMethod string
	Args          any
	Reply         any
	Error         error
	Done          chan *Call
}

func (call *Call) done() {
	call.Done <- call
}
