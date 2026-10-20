package ipc

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"

	"example.edu/6cc545/cw1/internal/config"
)

const MaxMessage = 4096
const Version = 1
const MaxTextBytes = 512

var ErrUnimplemented = errors.New("TODO Week 4: implement IPC")

type Message struct {
	Version   int    `json:"version"`
	RequestID string `json:"request_id"`
	NodeID    string `json:"node_id"`
	Text      string `json:"text"`
}

type Ack struct {
	Version   int    `json:"version"`
	RequestID string `json:"request_id"`
	Receiver  string `json:"receiver"`
	Accepted  bool   `json:"accepted"`
}

func (m Message) Validate() error {
	if m.Version != Version {
		return fmt.Errorf("unsupported message version")
	}
	if !config.ValidID(m.RequestID) {
		return fmt.Errorf("invalid request ID")
	}
	if !config.ValidID(m.NodeID) {
		return fmt.Errorf("invalid node ID")
	}
	if len(m.Text) == 0 || len(m.Text) > MaxTextBytes {
		return fmt.Errorf("text must contain 1 to 512 bytes")
	}
	if !utf8.ValidString(m.Text) {
		return fmt.Errorf("text must be valid UTF-8")
	}
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if len(data) > MaxMessage {
		return fmt.Errorf("encoded message exceeds limit")
	}
	return nil
}

func Encode(v any) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 || len(data) > MaxMessage {
		return nil, errors.New("message exceeds limit")
	}
	return data, nil
}

func Decode(data []byte, v any) error {
	if len(data) == 0 || len(data) > MaxMessage {
		return errors.New("message size is invalid")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(v); err != nil {
		return err
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func WriteFrame(w io.Writer, data []byte) error {
	if len(data) == 0 || len(data) > MaxMessage {
		return errors.New("frame size is invalid")
	}

	var header [4]byte
	binary.BigEndian.PutUint32(header[:], uint32(len(data)))

	if err := writeAll(w, header[:]); err != nil {
		return err
	}
	return writeAll(w, data)
}

func writeAll(w io.Writer, data []byte) error {
	for len(data) > 0 {
		n, err := w.Write(data)
		if err != nil {
			return err
		}
		if n <= 0 {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}

func ReadFrame(r io.Reader) ([]byte, error) {
	var header [4]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, err
	}

	size := binary.BigEndian.Uint32(header[:])
	if size == 0 || size > MaxMessage {
		return nil, errors.New("frame size is invalid")
	}

	data := make([]byte, int(size))
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, err
	}
	return data, nil
}
