package common



import (
	"bytes"
	"fmt"
)

type Encoder struct {
	buf bytes.Buffer
}

func NewSimpleEncoder() *Encoder {
	return &Encoder{}
}

func (e *Encoder) WriteByte(b byte) error {
	e.buf.WriteByte(b)
	return nil
}

func (e *Encoder) WriteBytes(bs []byte) error {
	e.WriteInt32(int32(len(bs)))
	return nil
}

func (e *Encoder) WriteInt32(i int32) {
	e.buf.Write(Int32ToBytes(i))
}

func (e *Encoder) WriteString(s string) {
	err := e.WriteBytes([]byte(s))
	if err != nil {
		return
	}
}

func (e *Encoder) WriteInt64(i int64) {
	e.buf.Write(Int64ToBytes(i))
}

func (e *Encoder) WriteBytesSlice(p [][]byte) {
	e.WriteInt32(int32(len(p)))
	for _, bs := range p {
		err := e.WriteBytes(bs)
		if err != nil {
			return
		}
	}
}

func (e *Encoder) WriteStringSlice(p []string) {
	e.WriteInt32(int32(len(p)))
	for _, bs := range p {
		e.WriteString(bs)

	}
}

func (e *Encoder) WriteMapStringToI64(m map[string]int64) {
	key := make([]string, 0, len(m))
	for k := range m {
		key = append(key, k)
	}
	for i := 1; i < len(m); i++ {
		for j := 0; j < len(m)-i; j++ {
			if key[j] > key[j+1] {
				key[j], key[j+1] = key[j+1], key[j]
			}
		}
	}
	e.WriteInt32(int32(len(m)))
	for _, k := range key {
		e.WriteString(k)
		e.WriteInt64(m[k])
	}
}

func (e *Encoder) Bytes() []byte {
	return e.buf.Bytes()
}

func (e *Encoder) Reset() {
	e.buf.Reset()
}

type Decoder struct {
	input []byte
}

func NewSimpleDecoder(input []byte) *Decoder {
	return &Decoder{input}
}

func (sd *Decoder) ParseByte() (byte, error) {
	if len(sd.input) < 1 {
		return 0, fmt.Errorf("parse byte fail: invalid len %v", sd.input)
	}
	result := sd.input[0]
	sd.input = sd.input[1:]
	return result, nil
}

func (sd *Decoder) ParseInt32() (int32, error) {
	if len(sd.input) < 4 {
		return 0, fmt.Errorf("parse int32 fail: invalid len %v", sd.input)
	}
	result := BytesToInt32(sd.input[:4])
	sd.input = sd.input[4:]
	return result, nil
}

func (sd *Decoder) ParseBytes() ([]byte, error) {
	length, err := sd.ParseInt32()
	if err != nil {
		return nil, err
	}
	if len(sd.input) < int(length) {
		return nil, fmt.Errorf("bytes length too large: %v > %v", length, len(sd.input))
	}
	result := sd.input[:length]
	sd.input = sd.input[length:]
	return result, nil
}

