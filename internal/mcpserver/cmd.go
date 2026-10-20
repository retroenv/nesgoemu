package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/retroenv/retrogolib/cli"
	"github.com/retroenv/retrogolib/log"
)

// Transport names.
const (
	transportStdio = "stdio"
	transportHTTP  = "http"
)

// shutdownTimeout bounds the graceful HTTP shutdown.
const shutdownTimeout = 5 * time.Second

// Main runs the MCP server command and returns an error when it fails.
func Main(args []string, version string) error {
	rootDirs, rest := extractRoots(args)

	var flags commandFlags
	flagSet := cli.NewFlagSet("nesgoemu mcp")
	flagSet.AddSection("Options", &flags)

	if _, err := flagSet.Parse(rest); err != nil {
		// The flag set prints the usage text by itself for -h.
		if errors.Is(err, cli.ErrHelpRequested) {
			return nil
		}
		return fmt.Errorf("reading options: %w", err)
	}

	machineTTL, err := time.ParseDuration(flags.MachineTTL)
	if err != nil {
		return fmt.Errorf("parsing machine-ttl: %w", err)
	}

	level := log.InfoLevel
	if flags.Debug {
		level = log.DebugLevel
	}
	logger := log.NewWithConfig(log.Config{
		Level:  level,
		Output: os.Stderr,
	})

	server, err := New(logger, Options{
		Name:        "nesgoemu",
		Version:     version,
		Roots:       rootDirs,
		MaxMachines: flags.MaxMachines,
		MachineTTL:  machineTTL,
	})
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch flags.Transport {
	case transportStdio:
		return server.Serve(ctx, os.Stdin, os.Stdout)

	case transportHTTP:
		return serveHTTP(ctx, logger, server, flags.Addr)

	default:
		return fmt.Errorf("unknown transport %q; use %s or %s", flags.Transport, transportStdio, transportHTTP)
	}
}

// commandFlags holds the options of the MCP server command.
type commandFlags struct {
	Transport   string `flag:"transport" usage:"transport: stdio or http" default:"stdio"`
	Addr        string `flag:"addr" usage:"listen address for the http transport" default:"127.0.0.1:0"`
	MaxMachines int    `flag:"max-machines" usage:"maximum number of live machines" default:"4"`
	MachineTTL  string `flag:"machine-ttl" usage:"idle limit for a machine, as a duration" default:"30m"`
	Debug       bool   `flag:"debug" usage:"enable debug logging"`

	// Root only documents the flag in the usage text. The flag is repeatable,
	// so extractRoots reads it before the flag set parses the other arguments.
	Root string `flag:"root" usage:"directory that holds ROMs (repeatable)"`
}

// extractRoots removes every -root flag and returns its values with the
// remaining arguments.
func extractRoots(args []string) ([]string, []string) {
	var roots, rest []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-root" || arg == "--root":
			if i+1 < len(args) {
				roots = append(roots, args[i+1])
				i++
			}

		case strings.HasPrefix(arg, "-root=") || strings.HasPrefix(arg, "--root="):
			roots = append(roots, arg[strings.IndexByte(arg, '=')+1:])

		default:
			rest = append(rest, arg)
		}
	}
	return roots, rest
}

// serveHTTP serves the streamable HTTP transport until the context ends.
func serveHTTP(ctx context.Context, logger *log.Logger, server *Server, addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", addr, err)
	}

	httpServer := &http.Server{
		Handler:           server.Handler(),
		ReadHeaderTimeout: shutdownTimeout,
	}
	logger.Info("MCP HTTP server listening", log.Stringer("address", listener.Addr()))

	serveErr := make(chan error, 1)
	go func() { serveErr <- httpServer.Serve(listener) }()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
		defer cancel()

		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutting down HTTP server: %w", err)
		}
		return nil

	case err := <-serveErr:
		return fmt.Errorf("serving HTTP: %w", err)
	}
}
