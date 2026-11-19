package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/retroenv/retrogolib/cli"
	"github.com/retroenv/retrogolib/log"
)

// Transport names.
const (
	transportStdio = "stdio"
	transportHTTP  = "http"
	rootFlagError  = "the -root flag needs a directory"
)

// shutdownTimeout bounds the graceful HTTP shutdown.
const shutdownTimeout = 5 * time.Second

// idleTimeout closes an idle HTTP connection.
const idleTimeout = 2 * time.Minute

// Main runs the MCP server command and returns an error when it fails.
func Main(ctx context.Context, args []string, version string) error {
	rootDirs, rest, err := extractRoots(args)
	if err != nil {
		return err
	}

	var flags commandFlags
	flagSet := cli.NewFlagSet("nesgoemu mcp")
	flagSet.AddSection("Options", &flags)

	positional, err := flagSet.Parse(rest)
	if err != nil {
		// The flag set prints the usage text by itself for -h.
		if errors.Is(err, cli.ErrHelpRequested) {
			return nil
		}
		return fmt.Errorf("reading options: %w", err)
	}
	if len(positional) > 0 {
		return fmt.Errorf("unexpected argument %q; use -root to name a ROM directory", positional[0])
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
		Name:    "nesgoemu",
		Version: version,

		MachineTTL:  machineTTL,
		MaxMachines: flags.MaxMachines,
		Roots:       rootDirs,
	})
	if err != nil {
		return err
	}

	switch flags.Transport {
	case transportStdio:
		return serveStdio(ctx, server)

	case transportHTTP:
		return serveHTTP(ctx, logger, server, flags.Addr)

	default:
		return fmt.Errorf("unknown transport %q; use %s or %s", flags.Transport, transportStdio, transportHTTP)
	}
}

// commandFlags holds the options of the MCP server command.
type commandFlags struct {
	Transport string `flag:"transport" usage:"transport: stdio or http" default:"stdio"`
	Addr      string `flag:"addr" usage:"listen address for the http transport" default:"127.0.0.1:0"`

	MaxMachines int    `flag:"max-machines" usage:"maximum number of live machines" default:"4"`
	MachineTTL  string `flag:"machine-ttl" usage:"idle limit for a machine, as a duration" default:"30m"`

	Debug bool `flag:"debug" usage:"enable debug logging"`

	// Root only documents the flag in the usage text. The flag is repeatable,
	// so extractRoots reads it before the flag set parses the other arguments.
	Root string `flag:"root" usage:"directory that holds ROMs (repeatable)"`
}

// extractRoots removes every -root flag and returns its values with the
// remaining arguments. A -root flag without a directory is an error, because
// the server would use the working directory instead.
func extractRoots(args []string) ([]string, []string, error) {
	var roots, rest []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-root" || arg == "--root":
			if i+1 >= len(args) {
				return nil, nil, errors.New(rootFlagError)
			}
			roots = append(roots, args[i+1])
			i++

		case strings.HasPrefix(arg, "-root=") || strings.HasPrefix(arg, "--root="):
			roots = append(roots, arg[strings.IndexByte(arg, '=')+1:])

		default:
			rest = append(rest, arg)
		}
	}

	for _, root := range roots {
		if strings.TrimSpace(root) == "" {
			return nil, nil, errors.New(rootFlagError)
		}
	}

	return roots, rest, nil
}

// serveStdio serves MCP over standard input and output. A blocked read of
// standard input does not return when the context ends, so the function
// returns on the signal without waiting for the serve loop.
func serveStdio(ctx context.Context, server *Server) error {
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx, os.Stdin, os.Stdout) }()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return nil
	}
}

// warnIfExposed logs a warning when the HTTP transport listens on an address
// that other hosts can reach. The transport has no authentication.
func warnIfExposed(logger *log.Logger, addr net.Addr) {
	tcp, ok := addr.(*net.TCPAddr)
	if ok && tcp.IP.IsLoopback() {
		return
	}

	logger.Warn("MCP HTTP server is reachable from other hosts and has no authentication",
		log.Stringer("address", addr))
}

// serveHTTP serves the streamable HTTP transport until the context ends.
func serveHTTP(ctx context.Context, logger *log.Logger, server *Server, addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", addr, err)
	}

	httpServer := &http.Server{
		Handler:           server.Handler(),
		ReadTimeout:       time.Minute,
		ReadHeaderTimeout: shutdownTimeout,
		IdleTimeout:       idleTimeout,
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}
	logger.Info("MCP HTTP server listening", log.Stringer("address", listener.Addr()))
	warnIfExposed(logger, listener.Addr())

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
