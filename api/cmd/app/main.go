// Command app runs the API. Configuration comes from the environment (see .env.example).
//
//	app               serve HTTP (same as `app serve`)
//	app migrate       apply pending database migrations as the owner role (MIGRATION_DATABASE_URL), then exit
//	app healthcheck   exit 0 if the local server answers /healthz (for container health checks)
//	app help          print usage
//
// Each subcommand parses its own flags. An unknown subcommand or flag exits with status 2.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/agusheryanto182/go-next-saas/api/config"
	"github.com/agusheryanto182/go-next-saas/api/internal/app"
	"github.com/agusheryanto182/go-next-saas/api/pkg/logger"
)

const _defaultCommand = "serve"

// command is one subcommand. setup defines the command's flags on its own flag set and returns
// the function that runs it once the flags are parsed.
type command struct {
	name    string
	summary string
	setup   func(fs *flag.FlagSet) func(stdout, stderr io.Writer) int
}

func commands() []command {
	return []command{
		{
			name:    "serve",
			summary: "serve HTTP (the default)",
			setup:   func(*flag.FlagSet) func(stdout, stderr io.Writer) int { return serve },
		},
		{
			name:    "migrate",
			summary: "apply pending migrations as the owner role (MIGRATION_DATABASE_URL), then exit",
			setup:   func(*flag.FlagSet) func(stdout, stderr io.Writer) int { return runMigrations },
		},
		{
			name:    "healthcheck",
			summary: "exit 0 if the local server answers /healthz (for container health checks)",
			setup: func(*flag.FlagSet) func(stdout, stderr io.Writer) int {
				return func(io.Writer, io.Writer) int { return checkHealth() }
			},
		},
	}
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run dispatches args (without the program name) to a subcommand and returns the exit status:
// 0 on success, 1 when the command fails, 2 for a usage error.
func run(args []string, stdout, stderr io.Writer) int {
	name, rest := _defaultCommand, args
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		name, rest = args[0], args[1:]
	}
	if name == "help" {
		if len(rest) > 0 {
			fmt.Fprintf(stderr, "app help: unexpected argument %q\n\n", rest[0])
			printUsage(stderr, nil)
			return 2
		}
		printUsage(stdout, nil)
		return 0
	}

	var cmd *command
	cmds := commands()
	for i := range cmds {
		if cmds[i].name == name {
			cmd = &cmds[i]
			break
		}
	}
	if cmd == nil {
		fmt.Fprintf(stderr, "app: unknown command %q\n\n", name)
		printUsage(stderr, nil)
		return 2
	}

	fs := flag.NewFlagSet("app "+name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {} // usage is printed below, to stdout for -h and to stderr for errors
	exec := cmd.setup(fs)
	if err := fs.Parse(rest); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printUsage(stdout, fs)
			return 0
		}
		fmt.Fprintln(stderr)
		printUsage(stderr, fs)
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "app %s: unexpected argument %q\n\n", name, fs.Arg(0))
		printUsage(stderr, fs)
		return 2
	}
	return exec(stdout, stderr)
}

// printUsage lists the subcommands and, when fs has any, the flags of the selected one.
func printUsage(w io.Writer, fs *flag.FlagSet) {
	fmt.Fprint(w, "Usage: app [command] [flags]\n\nCommands:\n")
	for _, c := range commands() {
		fmt.Fprintf(w, "  %-12s %s\n", c.name, c.summary)
	}
	fmt.Fprintf(w, "  %-12s %s\n", "help", "print this help")
	if fs == nil {
		return
	}
	hasFlags := false
	fs.VisitAll(func(*flag.Flag) { hasFlags = true })
	if hasFlags {
		fmt.Fprintf(w, "\nFlags for %s:\n", fs.Name())
		fs.SetOutput(w) // parsing is over; the flag set's output is only used here
		fs.PrintDefaults()
	}
}

func runMigrations(stdout, _ io.Writer) int {
	log := logger.New(stdout, slog.LevelInfo)
	url := os.Getenv("MIGRATION_DATABASE_URL")
	if url == "" {
		log.Error("MIGRATION_DATABASE_URL is not set (the owner role's connection URL)")
		return 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := app.Migrate(ctx, url, log); err != nil {
		log.Error("migration failed", slog.Any("error", err))
		return 1
	}
	log.Info("database is up to date")
	return 0
}

func serve(stdout, stderr io.Writer) int {
	cfg, err := config.New()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	log := logger.New(stdout, cfg.SlogLevel())
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := app.Run(ctx, cfg, log); err != nil {
		log.Error("api failed", slog.Any("error", err))
		return 1
	}
	return 0
}

// checkHealth calls /healthz on the port from HTTP_ADDR. The runtime image has no shell or curl.
func checkHealth() int {
	port := 8080
	if _, p, err := net.SplitHostPort(os.Getenv("HTTP_ADDR")); err == nil {
		if n, err := strconv.Atoi(p); err == nil && n > 0 && n < 65536 {
			port = n
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	url := fmt.Sprintf("http://127.0.0.1:%d/healthz", port)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil) //nolint:gosec // loopback host, numeric port
	if err != nil {
		return 1
	}
	res, err := http.DefaultClient.Do(req) //nolint:gosec // loopback host, numeric port
	if err != nil {
		return 1
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}
