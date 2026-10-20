package ipc

import (
	"context"
	"errors"
	"example.edu/6cc545/cw1/internal/config"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

type OnMessage func(Message)

func deadline(ctx context.Context, timeout time.Duration) time.Time {
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	d := time.Now().Add(timeout)
	if cd, ok := ctx.Deadline(); ok && cd.Before(d) {
		return cd
	}
	return d
}

func makeAck(id string, m Message) Ack {
	return Ack{
		Version:   Version,
		RequestID: m.RequestID,
		Receiver:  id,
		Accepted:  true,
	}
}

func checkAck(a Ack, m Message) error {
	if a.Version != Version ||
		a.RequestID != m.RequestID ||
		!a.Accepted ||
		!config.ValidID(a.Receiver) {
		return errors.New("invalid or unrelated acknowledgement")
	}
	return nil
}

func configValidID(s string) bool {
	return len(s) > 0 && len(s) <= 32 &&
		!containsInvalidIDCharacter(s)
}

func containsInvalidIDCharacter(s string) bool {
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '_' || r == '-') {
			return true
		}
	}
	return false
}

func ServeTCP(ctx context.Context, ln net.Listener, id string, timeout time.Duration, on OnMessage) error {
	if timeout <= 0 {
		timeout = 2 * time.Second
	}

	stopClose := context.AfterFunc(ctx, func() {
		_ = ln.Close()
	})
	defer stopClose()

	var wg sync.WaitGroup
	sem := make(chan struct{}, 32)

	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				wg.Wait()
				return nil
			}
			return fmt.Errorf("accept TCP connection: %w", err)
		}

		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			_ = conn.Close()
			wg.Wait()
			return nil
		}

		wg.Add(1)
		go func(c net.Conn) {
			defer wg.Done()
			defer func() { <-sem }()
			defer c.Close()

			_ = c.SetDeadline(deadline(ctx, timeout))

			data, err := ReadFrame(c)
			if err != nil {
				return
			}

			var m Message
			if Decode(data, &m) != nil || m.Validate() != nil {
				return
			}

			if on != nil {
				on(m)
			}

			ackData, err := Encode(makeAck(id, m))
			if err != nil {
				return
			}
			_ = WriteFrame(c, ackData)
		}(conn)
	}
}

func SendTCP(ctx context.Context, address string, m Message, timeout time.Duration) (Ack, error) {
	if err := ctx.Err(); err != nil {
		return Ack{}, err
	}
	if err := m.Validate(); err != nil {
		return Ack{}, err
	}

	ctx, cancel := context.WithDeadline(ctx, deadline(ctx, timeout))
	defer cancel()

	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", address)
	if err != nil {
		return Ack{}, err
	}
	defer conn.Close()

	stopClose := context.AfterFunc(ctx, func() {
		_ = conn.Close()
	})
	defer stopClose()

	if err := conn.SetDeadline(deadline(ctx, timeout)); err != nil {
		return Ack{}, err
	}

	data, err := Encode(m)
	if err != nil {
		return Ack{}, err
	}
	if err := WriteFrame(conn, data); err != nil {
		return Ack{}, err
	}

	reply, err := ReadFrame(conn)
	if err != nil {
		return Ack{}, err
	}

	var ack Ack
	if err := Decode(reply, &ack); err != nil {
		return Ack{}, err
	}
	if err := checkAck(ack, m); err != nil {
		return Ack{}, err
	}
	return ack, nil
}

func ServeUDP(ctx context.Context, c *net.UDPConn, id string, timeout time.Duration, on OnMessage) error {
	if timeout <= 0 {
		timeout = 2 * time.Second
	}

	stopClose := context.AfterFunc(ctx, func() {
		_ = c.Close()
	})
	defer stopClose()

	buffer := make([]byte, MaxMessage+1)

	for {
		n, addr, err := c.ReadFromUDP(buffer)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("read UDP datagram: %w", err)
		}

		if n == 0 || n > MaxMessage {
			continue
		}

		var m Message
		if Decode(buffer[:n], &m) != nil || m.Validate() != nil {
			continue
		}

		if on != nil {
			on(m)
		}

		ackData, err := Encode(makeAck(id, m))
		if err != nil {
			continue
		}

		_ = c.SetWriteDeadline(time.Now().Add(timeout))
		_, _ = c.WriteToUDP(ackData, addr)
	}
}

func SendUDP(ctx context.Context, address string, m Message, timeout time.Duration) (Ack, error) {
	if err := ctx.Err(); err != nil {
		return Ack{}, err
	}
	if err := m.Validate(); err != nil {
		return Ack{}, err
	}

	ctx, cancel := context.WithDeadline(ctx, deadline(ctx, timeout))
	defer cancel()

	remote, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return Ack{}, err
	}

	conn, err := net.DialUDP("udp", nil, remote)
	if err != nil {
		return Ack{}, err
	}
	defer conn.Close()

	stopClose := context.AfterFunc(ctx, func() {
		_ = conn.Close()
	})
	defer stopClose()

	if err := conn.SetDeadline(deadline(ctx, timeout)); err != nil {
		return Ack{}, err
	}

	data, err := Encode(m)
	if err != nil {
		return Ack{}, err
	}

	n, err := conn.Write(data)
	if err != nil {
		return Ack{}, err
	}
	if n != len(data) {
		return Ack{}, io.ErrShortWrite
	}

	buffer := make([]byte, MaxMessage+1)
	n, err = conn.Read(buffer)
	if err != nil {
		return Ack{}, err
	}
	if n == 0 || n > MaxMessage {
		return Ack{}, errors.New("invalid acknowledgement size")
	}

	var ack Ack
	if err := Decode(buffer[:n], &ack); err != nil {
		return Ack{}, err
	}
	if err := checkAck(ack, m); err != nil {
		return Ack{}, err
	}
	return ack, nil
}
