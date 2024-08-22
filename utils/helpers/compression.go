package helpers

import (
	"bytes"
	"io"

	"github.com/icza/huffman/hufio"
)

func Compress(data []byte) ([]byte, error) {
	buf := &bytes.Buffer{}
	w := hufio.NewWriter(buf)

	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func Decompress(data []byte) ([]byte, error) {
	r := hufio.NewReader(bytes.NewReader(data))
	if data, err := io.ReadAll(r); err != nil {
		return nil, err
	} else {
		return data, nil
	}
}
