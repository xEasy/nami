package codec

import (
	"bufio"
	"encoding/gob"
	"fmt"
	"io"
)

type GobCodec struct {
	conn io.ReadWriteCloser
	buf  *bufio.Writer
	dec  *gob.Decoder
	enc  *gob.Encoder
}

func NewGobCodec(conn io.ReadWriteCloser) Codec {
	buf := bufio.NewWriter(conn)
	return &GobCodec{
		conn: conn,
		buf:  buf,
		dec:  gob.NewDecoder(conn),
		enc:  gob.NewEncoder(buf),
	}
}

func (g *GobCodec) ReadHeader(header *Header) error {
	return g.dec.Decode(header)
}

func (g *GobCodec) ReadBody(body interface{}) error {
	return g.dec.Decode(body)
}

func (g *GobCodec) Write(header *Header, body interface{}) error {
	defer func() {
		err := g.buf.Flush()
		if err != nil {
			g.Close()
		}
	}()

	if err := g.enc.Encode(header); err != nil {
		fmt.Println("rpc codec: gob error encoding header: ", err)
		return err
	}

	if err := g.enc.Encode(body); err != nil {
		fmt.Println("rpc codec: gob error encoding body: ", err)
		return err
	}

	return nil
}

func (g *GobCodec) Close() error {
	return g.conn.Close()
}
