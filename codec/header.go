package codec

type Header struct {
	ServiceMethod string
	Seq           uint64
	Error         string
}
