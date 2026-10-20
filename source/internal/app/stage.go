package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"sync"

	"example.edu/6cc545/cw1/internal/config"
	"example.edu/6cc545/cw1/internal/httpapi"
	"example.edu/6cc545/cw1/internal/ipc"
	"example.edu/6cc545/cw1/internal/model"
	"example.edu/6cc545/cw1/internal/state"
)

func CheckStage(c config.Config) error {
	if c.GRPC != "" {
		return errors.New("Week 5: gRPC is not enabled yet")
	}
	return nil
}

func loadSeed(c *state.Catalog, path string) error {
	if path == "" {
		return nil
	}

	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open seed file: %w", err)
	}
	defer f.Close()

	data, err := io.ReadAll(io.LimitReader(f, 65537))
	if err != nil {
		return fmt.Errorf("read seed file: %w", err)
	}
	if len(data) > 65536 {
		return errors.New("seed file exceeds 65536 bytes")
	}

	var files []model.FileMetadata
	if err := json.Unmarshal(data, &files); err != nil {
		return fmt.Errorf("decode seed file: %w", err)
	}
	if err := c.Replace(files); err != nil {
		return fmt.Errorf("validate seed file: %w", err)
	}
	return nil
}

func Wire(ctx context.Context, c config.Config, mux *http.ServeMux) (func(), error) {
	status := state.NewStatus()
	registry := state.NewRegistry()
	catalog := state.NewCatalog()
	inbox := state.NewInbox()

	if err := loadSeed(catalog, c.Seed); err != nil {
		return nil, err
	}

	var tcpListener net.Listener
	var udpConn *net.UDPConn

	if c.TCP != "" {
		ln, err := net.Listen("tcp", c.TCP)
		if err != nil {
			return nil, fmt.Errorf("listen TCP %s: %w", c.TCP, err)
		}
		tcpListener = ln
	}

	if c.UDP != "" {
		addr, err := net.ResolveUDPAddr("udp", c.UDP)
		if err != nil {
			if tcpListener != nil {
				_ = tcpListener.Close()
			}
			return nil, fmt.Errorf("resolve UDP %s: %w", c.UDP, err)
		}

		conn, err := net.ListenUDP("udp", addr)
		if err != nil {
			if tcpListener != nil {
				_ = tcpListener.Close()
			}
			return nil, fmt.Errorf("listen UDP %s: %w", c.UDP, err)
		}
		udpConn = conn
	}

	mux.HandleFunc("/v1/status", httpapi.StatusResource(status))
	mux.HandleFunc("/v1/peers", httpapi.Peers(registry))
	mux.HandleFunc("/v1/files", httpapi.Files(catalog))
	mux.HandleFunc("/v1/messages", httpapi.Messages(inbox))

	runCtx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	var stopOnce sync.Once

	stop := func() {
		stopOnce.Do(func() {
			cancel()
			if tcpListener != nil {
				_ = tcpListener.Close()
			}
			if udpConn != nil {
				_ = udpConn.Close()
			}
			wg.Wait()
		})
	}

	if tcpListener != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := ipc.ServeTCP(runCtx, tcpListener, c.ID, c.Timeout, inbox.Record); err != nil {
				log.Printf("TCP server stopped: %v", err)
			}
		}()
		log.Printf("node=%s tcp=%s ready", c.ID, tcpListener.Addr())
	}

	if udpConn != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := ipc.ServeUDP(runCtx, udpConn, c.ID, c.Timeout, inbox.Record); err != nil {
				log.Printf("UDP server stopped: %v", err)
			}
		}()
		log.Printf("node=%s udp=%s ready", c.ID, udpConn.LocalAddr())
	}

	return stop, nil
}
