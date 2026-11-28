package app

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	pb "example.edu/6cc545/cw1/api/files/v1"
	"example.edu/6cc545/cw1/internal/rpcfiles"
	"example.edu/6cc545/cw1/internal/security"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"example.edu/6cc545/cw1/internal/config"
	"example.edu/6cc545/cw1/internal/events"
	"example.edu/6cc545/cw1/internal/httpapi"
	"example.edu/6cc545/cw1/internal/ipc"
	"example.edu/6cc545/cw1/internal/membership"
	"example.edu/6cc545/cw1/internal/model"
	"example.edu/6cc545/cw1/internal/state"
	"example.edu/6cc545/cw1/internal/storage"
)

func CheckStage(c config.Config) error {
	if c.SecurityMode != "required" {
		return nil
	}

	if _, err := security.ServerTLS(
		c.TLSCertFile,
		c.TLSKeyFile,
		c.CAFile,
		false,
	); err != nil {
		return fmt.Errorf("check TLS configuration: %w", err)
	}

	key, err := os.ReadFile(c.JWTKeyFile)
	if err != nil {
		return fmt.Errorf("read JWT signing key: %w", err)
	}
	if len(key) < 32 {
		return fmt.Errorf("JWT signing key must contain at least 32 bytes")
	}

	return nil
}

// peerFromSeed converts a Compose gRPC seed into the peer HTTP and gRPC
// service addresses. The Week 8 Compose network uses HTTP port 8080.
func peerFromSeed(seed string) (model.Peer, error) {
	if strings.HasPrefix(seed, "https://") {
		parsed, err := url.Parse(seed)
		if err != nil || !config.ValidPeerHTTPSURL(seed) {
			return model.Peer{}, fmt.Errorf("invalid secure peer seed %q", seed)
		}

		host := parsed.Hostname()
		if !config.ValidID(host) {
			return model.Peer{}, fmt.Errorf("invalid peer seed %q: host must be a valid node ID", seed)
		}

		return model.Peer{
			ID:          host,
			HTTPAddress: seed,
			GRPCAddress: net.JoinHostPort(host, "9443"),
			Status:      "unknown",
		}, nil
	}

	host, _, err := net.SplitHostPort(seed)
	if err != nil || !config.ValidID(host) {
		return model.Peer{}, fmt.Errorf("invalid peer seed %q: host must be a valid node ID", seed)
	}

	return model.Peer{
		ID:          host,
		HTTPAddress: net.JoinHostPort(host, "8080"),
		GRPCAddress: seed,
		Status:      "unknown",
	}, nil
}

func Wire(ctx context.Context, c config.Config, mux *http.ServeMux) (func(), error) {
	status := state.NewStatus()
	registry := state.NewRegistry()
	inbox := state.NewInbox()

	for _, seed := range c.PeerSeeds {
		peer, err := peerFromSeed(seed)
		if err != nil {
			return nil, err
		}
		if peer.ID == c.ID {
			return nil, fmt.Errorf("peer seed %q refers to this node", seed)
		}
		if err := registry.Add(peer); err != nil {
			return nil, fmt.Errorf("register peer seed %q: %w", seed, err)
		}
	}

	var grpcJWTValidator security.TokenValidator
	if c.SecurityMode == "required" {
		jwtKey, err := os.ReadFile(c.JWTKeyFile)
		if err != nil {
			return nil, fmt.Errorf("read gRPC JWT signing key: %w", err)
		}
		grpcJWTValidator = security.TokenValidator{
			Key:      jwtKey,
			Issuer:   c.JWTIssuer,
			Audience: c.JWTAudience,
		}
	}

	var tcpListener net.Listener
	var udpConn *net.UDPConn
	var grpcListener net.Listener

	var grpcTLSConfig *tls.Config
	if c.SecurityMode == "required" {
		var err error
		grpcTLSConfig, err = security.ServerTLS(
			c.TLSCertFile,
			c.TLSKeyFile,
			c.CAFile,
			true,
		)
		if err != nil {
			return nil, fmt.Errorf("configure gRPC mutual TLS: %w", err)
		}
	}

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
	const maxStoredFileBytes int64 = 10 << 20
	dataDir := c.DataDir
	if dataDir == "" {
		dataDir = filepath.Join("data", c.ID)
	}

	store, err := storage.New(dataDir, maxStoredFileBytes)
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
		return nil, fmt.Errorf("create file storage: %w", err)
	}

	// Enable durable file versioning for secure CW2 nodes before recovery.
	if c.SecurityMode == "required" {
		if err := store.EnableVersioning(c.ID); err != nil {
			if tcpListener != nil {
				_ = tcpListener.Close()
			}
			if udpConn != nil {
				_ = udpConn.Close()
			}
			if grpcListener != nil {
				_ = grpcListener.Close()
			}
			return nil, fmt.Errorf("enable versioned file storage: %w", err)
		}
	}

	// Restore committed files before the application starts serving requests.
	if err := store.Recover(ctx); err != nil {
		if tcpListener != nil {
			_ = tcpListener.Close()
		}
		if udpConn != nil {
			_ = udpConn.Close()
		}
		if grpcListener != nil {
			_ = grpcListener.Close()
		}
		return nil, fmt.Errorf("recover file storage: %w", err)
	}

	var membershipTable *membership.Table
	if c.SecurityMode == "required" {
		membershipTable, err = membership.NewPersistent(
			128,
			4*time.Second,
			filepath.Join(dataDir, ".cw2-state", "membership.json"),
		)
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
			return nil, fmt.Errorf("restore persistent peer membership: %w", err)
		}
	}

	storageCatalog, err := rpcfiles.NewStorageCatalog(store)
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
		return nil, fmt.Errorf("create storage catalogue: %w", err)
	}

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
	mux.HandleFunc("/v1/files", httpapi.StoredFiles(store))
	mux.HandleFunc("/v1/files/", httpapi.StoredFile(store, eventBus, c.ID, maxStoredFileBytes))
	mux.HandleFunc("/v1/messages", httpapi.Messages(inbox))
	mux.HandleFunc("/v1/remote-files", httpapi.RemoteFiles(registry, c.ID, c.Timeout, c))
	mux.HandleFunc("/v1/events", httpapi.Events(eventBus))

	runCtx, cancel := context.WithCancel(ctx)

	var peerHTTPListener net.Listener
	var peerHTTPServer *http.Server
	var peerHTTPDone chan struct{}
	var peerTLSConfig *tls.Config

	if c.SecurityMode == "required" {
		var err error
		peerTLSConfig, err = security.ServerTLS(
			c.TLSCertFile,
			c.TLSKeyFile,
			c.CAFile,
			true,
		)
		if err != nil {
			cancel()
			auditSub.Cancel()
			metricsSub.Cancel()
			forwardSub.Cancel()
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
			return nil, fmt.Errorf("configure private peer HTTPS: %w", err)
		}

		peerHTTPListener, err = net.Listen("tcp", c.PeerHTTP)
		if err != nil {
			cancel()
			auditSub.Cancel()
			metricsSub.Cancel()
			forwardSub.Cancel()
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
			return nil, fmt.Errorf("listen private peer HTTPS %s: %w", c.PeerHTTP, err)
		}

		peerMux := http.NewServeMux()
		peerMux.HandleFunc("/v1/peer/announce", httpapi.PeerAnnounce(eventBus, registry))
		peerMux.HandleFunc("/v1/peer/membership", httpapi.PeerMembership(membershipTable, registry))
		peerMux.HandleFunc("/v1/peer/files/", httpapi.PeerFiles(store, registry, maxStoredFileBytes))

		peerHTTPServer = &http.Server{
			Handler:           peerMux,
			ReadHeaderTimeout: c.Timeout,
			ReadTimeout:       2 * c.Timeout,
			WriteTimeout:      2 * c.Timeout,
			IdleTimeout:       30 * time.Second,
		}
	}

	var wg sync.WaitGroup
	var stopOnce sync.Once

	// Expire inactive authenticated peers and update their registry status.
	if membershipTable != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()

			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-runCtx.Done():
					return
				case now := <-ticker.C:
					for _, peerID := range membershipTable.Expire(now) {
						if err := registry.SetStatus(peerID, "offline"); err != nil {
							log.Printf("membership expiry status update failed for peer %s: %v", peerID, err)
						}
					}
				}
			}
		}()
	}

	// Announce this node's membership to its enrolled peers.
	if membershipTable != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runMembershipAnnouncements(
				runCtx,
				registry,
				c.ID,
				c.PeerHTTPAdvertise,
				membershipTable,
				c,
			)
		}()
	}

	// Reconcile authenticated peers using bounded, cancellable pull-based sync.
	if membershipTable != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runReconciliation(
				runCtx,
				c,
				store,
				registry,
				membershipTable,
				grpcJWTValidator,
			)
		}()
	}

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
					return forwardEventToPeers(sendCtx, registry, c.ID, event, c)
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
		grpcOptions := []grpc.ServerOption{
			grpc.MaxRecvMsgSize(4096),
			grpc.MaxSendMsgSize(65536),
		}
		if grpcTLSConfig != nil {
			grpcOptions = append(
				grpcOptions,
				grpc.Creds(credentials.NewTLS(grpcTLSConfig)),
				grpc.UnaryInterceptor(security.GRPCUnary(grpcJWTValidator, "reader")),
			)
		}

		grpcServer = grpc.NewServer(grpcOptions...)
		pb.RegisterFileServiceServer(grpcServer, &rpcfiles.Service{
			NodeID:              c.ID,
			Catalog:             storageCatalog,
			RequirePeerIdentity: c.SecurityMode == "required",
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

	if peerHTTPListener != nil {
		peerHTTPDone = make(chan struct{})
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer close(peerHTTPDone)

			listener := tls.NewListener(peerHTTPListener, peerTLSConfig)
			if err := peerHTTPServer.Serve(listener); err != nil && runCtx.Err() == nil {
				log.Printf("private peer HTTPS server stopped: %v", err)
			}
		}()
		log.Printf("node=%s peer-https=%s ready", c.ID, peerHTTPListener.Addr())
	}

	stop := func() {
		stopOnce.Do(func() {
			cancel()

			if peerHTTPServer != nil {
				shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), c.Timeout)
				err := peerHTTPServer.Shutdown(shutdownCtx)
				shutdownCancel()
				if err != nil {
					_ = peerHTTPServer.Close()
				}
				<-peerHTTPDone
			}

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

	return stop, nil
}
