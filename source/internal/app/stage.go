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
	"time"

	pb "example.edu/6cc545/cw1/api/files/v1"
	"example.edu/6cc545/cw1/internal/rpcfiles"
	"google.golang.org/grpc"

	"example.edu/6cc545/cw1/internal/config"
	"example.edu/6cc545/cw1/internal/events"
	"example.edu/6cc545/cw1/internal/httpapi"
	"example.edu/6cc545/cw1/internal/ipc"
	"example.edu/6cc545/cw1/internal/model"
	"example.edu/6cc545/cw1/internal/state"
)

func CheckStage(c config.Config) error {
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
	var grpcListener net.Listener

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

	if c.GRPC != "" {
		ln, err := net.Listen("tcp", c.GRPC)
		if err != nil {
			if tcpListener != nil {
				_ = tcpListener.Close()
			}
			if udpConn != nil {
				_ = udpConn.Close()
			}
			return nil, fmt.Errorf("listen gRPC %s: %w", c.GRPC, err)
		}
		grpcListener = ln
	}

	// One bounded event bus is shared by this node's subscribers.
	eventBus, err := events.New(64)
	if err != nil {
		if tcpListener != nil {
			_ = tcpListener.Close()
		}
		if udpConn != nil {
			_ = udpConn.Close()
		}
		if grpcListener != nil {
			_ = grpcListener.Close()
		}
		return nil, fmt.Errorf("create event bus: %w", err)
	}

	auditSub, err := eventBus.Subscribe("file.added")
	if err != nil {
		_ = eventBus.Close()
		if tcpListener != nil {
			_ = tcpListener.Close()
		}
		if udpConn != nil {
			_ = udpConn.Close()
		}
		if grpcListener != nil {
			_ = grpcListener.Close()
		}
		return nil, fmt.Errorf("create audit subscription: %w", err)
	}

	metricsSub, err := eventBus.Subscribe("file.added")
	if err != nil {
		auditSub.Cancel()
		_ = eventBus.Close()
		if tcpListener != nil {
			_ = tcpListener.Close()
		}
		if udpConn != nil {
			_ = udpConn.Close()
		}
		if grpcListener != nil {
			_ = grpcListener.Close()
		}
		return nil, fmt.Errorf("create metrics subscription: %w", err)
	}

	forwardSub, err := eventBus.Subscribe("file.added")
	if err != nil {
		auditSub.Cancel()
		metricsSub.Cancel()
		_ = eventBus.Close()
		if tcpListener != nil {
			_ = tcpListener.Close()
		}
		if udpConn != nil {
			_ = udpConn.Close()
		}
		if grpcListener != nil {
			_ = grpcListener.Close()
		}
		return nil, fmt.Errorf("create forwarding subscription: %w", err)
	}

	mux.HandleFunc("/v1/status", httpapi.StatusResource(status))
	mux.HandleFunc("/v1/peers", httpapi.Peers(registry))
	mux.HandleFunc("/v1/files", httpapi.Files(catalog))
	mux.HandleFunc("/v1/messages", httpapi.Messages(inbox))
	mux.HandleFunc("/v1/remote-files", httpapi.RemoteFiles(registry, c.ID, c.Timeout))
	mux.HandleFunc("/v1/events", httpapi.Events(eventBus))

	runCtx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	var stopOnce sync.Once

	// Both subscribers have their own queue and receive each matching event.
	wg.Add(2)
	go func() {
		defer wg.Done()
		defer auditSub.Cancel()

		for {
			select {
			case <-runCtx.Done():
				return
			case event, ok := <-auditSub.C:
				if !ok {
					return
				}
				log.Printf(
					"event audit id=%s kind=%s origin=%s name=%s",
					event.ID, event.Kind, event.Origin, event.Name,
				)
			}
		}
	}()

	go func() {
		defer wg.Done()
		defer metricsSub.Cancel()

		count := 0
		for {
			select {
			case <-runCtx.Done():
				log.Printf("event metrics file.added received=%d", count)
				return
			case event, ok := <-metricsSub.C:
				if !ok {
					log.Printf("event metrics file.added received=%d", count)
					return
				}
				count++
				log.Printf("event metrics kind=%s total=%d", event.Kind, count)
			}
		}
	}()

	// Forward file-added events to registered peers.
	wg.Add(1)
	go func() {
		defer wg.Done()
		sub := forwardSub
		for {
			err := eventBus.ForwardSubscription(
				runCtx,
				sub,
				func(sendCtx context.Context, event events.Event) error {
					return forwardEventToPeers(sendCtx, registry, c.ID, event)
				},
			)
			if runCtx.Err() != nil {
				return
			}
			if err != nil {
				log.Printf("event forwarding failed; restarting subscription: %v", err)
			}
			sub, err = eventBus.Subscribe("file.added")
			if err != nil {
				if runCtx.Err() == nil {
					log.Printf("event forwarding subscription stopped: %v", err)
				}
				return
			}
		}
	}()

	var grpcServer *grpc.Server
	var grpcDone chan struct{}

	if grpcListener != nil {
		grpcServer = grpc.NewServer(
			grpc.MaxRecvMsgSize(4096),
			grpc.MaxSendMsgSize(65536),
		)
		pb.RegisterFileServiceServer(grpcServer, &rpcfiles.Service{
			NodeID:  c.ID,
			Catalog: catalog,
		})

		grpcDone = make(chan struct{})
		go func() {
			defer close(grpcDone)
			if err := grpcServer.Serve(grpcListener); err != nil && runCtx.Err() == nil {
				log.Printf("gRPC server stopped: %v", err)
			}
		}()
		log.Printf("node=%s grpc=%s ready", c.ID, grpcListener.Addr())
	}

	stop := func() {
		stopOnce.Do(func() {
			cancel()
			_ = eventBus.Close()

			if tcpListener != nil {
				_ = tcpListener.Close()
			}
			if udpConn != nil {
				_ = udpConn.Close()
			}

			if grpcServer != nil {
				stopped := make(chan struct{})
				go func() {
					grpcServer.GracefulStop()
					close(stopped)
				}()

				select {
				case <-stopped:
				case <-time.After(c.Timeout):
					grpcServer.Stop()
					<-stopped
				}

				_ = grpcListener.Close()
				<-grpcDone
			}

			wg.Wait()
		})
	}

	if tcpListener != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := ipc.ServeTCP(runCtx, tcpListener, c.ID, c.Timeout, inbox.Record); err != nil && runCtx.Err() == nil {
				log.Printf("TCP server stopped: %v", err)
			}
		}()
		log.Printf("node=%s tcp=%s ready", c.ID, tcpListener.Addr())
	}

	if udpConn != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := ipc.ServeUDP(runCtx, udpConn, c.ID, c.Timeout, inbox.Record); err != nil && runCtx.Err() == nil {
				log.Printf("UDP server stopped: %v", err)
			}
		}()
		log.Printf("node=%s udp=%s ready", c.ID, udpConn.LocalAddr())
	}

	// Publish metadata events for files already present in the seed catalogue.
	for i, file := range catalog.Snapshot() {
		event := events.Event{
			ID:     fmt.Sprintf("seed-%d", i+1),
			Kind:   "file.added",
			Origin: c.ID,
			Name:   file.Name,
		}

		delivery, err := eventBus.Publish(event)
		if err != nil {
			log.Printf("event publish failed name=%s: %v", file.Name, err)
			continue
		}
		if delivery.Dropped > 0 {
			log.Printf(
				"event publish dropped name=%s subscribers=%d",
				file.Name, delivery.Dropped,
			)
		}
	}

	return stop, nil
}
