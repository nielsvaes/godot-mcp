package daemon

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tomyud1/godot-mcp/cli/internal/bridge"
	"github.com/tomyud1/godot-mcp/cli/internal/config"
	"github.com/tomyud1/godot-mcp/cli/internal/schema"
)

// Serve runs the daemon: WebSocket bridge + HTTP control API, with idle
// shutdown and signal handling. It blocks until shutdown.
func Serve() error {
	tools, err := schema.All()
	if err != nil {
		return err
	}

	b := bridge.New(config.WebSocketPort(), config.ToolTimeout())
	if err := b.Start(); err != nil {
		return fmt.Errorf("websocket bridge on :%d: %w", config.WebSocketPort(), err)
	}
	log.Printf("gdcli: bridge listening on :%d", config.WebSocketPort())

	h := NewHandler(b, config.Version, len(tools))
	httpLn, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", config.HTTPPort()))
	if err != nil {
		b.Stop()
		return fmt.Errorf("http api on :%d: %w", config.HTTPPort(), err)
	}
	httpSrv := &http.Server{Handler: h}
	log.Printf("gdcli: http api listening on :%d", config.HTTPPort())

	ctx, cancel := context.WithCancel(context.Background())
	h.SetShutdown(cancel)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		cancel()
	}()

	go func() { _ = httpSrv.Serve(httpLn) }()

	// Idle-shutdown watcher: exit when no Godot and no recent HTTP activity.
	idle := config.IdleTimeout()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Printf("gdcli: shutting down")
			_ = httpSrv.Close()
			b.Stop()
			return nil
		case <-ticker.C:
			if !b.Connected() && time.Since(h.LastActivity()) > idle {
				log.Printf("gdcli: idle for %s with no Godot connection; shutting down", idle)
				cancel()
			}
		}
	}
}
