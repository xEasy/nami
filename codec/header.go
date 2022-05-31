package codec

type Header struct {
	ServiceMethod string
	seq           uint64
	Error         string
}
