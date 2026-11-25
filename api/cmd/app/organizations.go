package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"golang.org/x/term"

	"github.com/agusheryanto182/go-next-saas/api/internal/app"
	"github.com/agusheryanto182/go-next-saas/api/internal/usecase"
	"github.com/agusheryanto182/go-next-saas/api/pkg/logger"
)

var (
	errPasswordMismatch = errors.New("the passwords do not match")
	errPasswordEmpty    = errors.New("the password is empty")
)

// seed runs `app seed`: the development dataset, as the app role.
func seed(stdout, _ io.Writer) int {
	log := logger.New(stdout, slog.LevelInfo)
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		log.Error("DATABASE_URL is not set (the app role's connection URL)")
		return 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := app.Seed(ctx, url, log); err != nil {
		log.Error("seed failed", slog.Any("error", err))
		return 1
	}
	log.Info("development data is in place")
	return 0
}

// setupCreateOrg defines the flags of `app create-org` (ADR 0012).
func setupCreateOrg(fs *flag.FlagSet) func(stdout, stderr io.Writer) int {
	name := fs.String("name", "", "organization name (required)")
	slug := fs.String("slug", "", "organization URL name, 3 to 40 of a-z, 0-9 and - (required)")
	email := fs.String("owner-email", "", "owner's email; an existing account is reused and keeps its password (required)")
	ownerName := fs.String("owner-name", "", "owner's full name, used when the account is new (required)")
	passwordStdin := fs.Bool("owner-password-stdin", false,
		"read the owner's password from standard input (no echo on a terminal); for the operator's own account")

	return func(stdout, stderr io.Writer) int {
		required := []struct{ flag, value string }{
			{"name", *name}, {"slug", *slug}, {"owner-email", *email}, {"owner-name", *ownerName},
		}
		for _, r := range required {
			if r.value == "" {
				fmt.Fprintf(stderr, "app create-org: -%s is required\n\n", r.flag)
				printUsage(stderr, fs)
				return 2
			}
		}
		if !*passwordStdin {
			fmt.Fprintln(stderr, "app create-org: invitations arrive in M2; use -owner-password-stdin")
			return 2
		}
		url := os.Getenv("DATABASE_URL")
		if url == "" {
			fmt.Fprintln(stderr, "app create-org: DATABASE_URL is not set (the app role's connection URL)")
			return 1
		}
		password, err := readPassword(os.Stdin, stderr)
		if err != nil {
			fmt.Fprintf(stderr, "app create-org: %v\n", err)
			return 1
		}
		return createOrg(stdout, stderr, url, usecase.CreateOrganizationInput{
			Name: *name, Slug: *slug,
			Owner: usecase.NewUser{Email: *email, FullName: *ownerName, Password: password},
		})
	}
}

func createOrg(stdout, stderr io.Writer, url string, in usecase.CreateOrganizationInput) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	res, err := app.CreateOrganization(ctx, url, in)
	if err != nil {
		fmt.Fprintf(stderr, "app create-org: %v\n", err)
		return 1
	}
	if res.Created {
		fmt.Fprintf(stdout, "Organization %q created (id %s).\n", in.Slug, res.Organization.ID)
	} else {
		fmt.Fprintf(stdout, "Organization %q already exists; nothing changed.\n", in.Slug)
	}
	return 0
}

// readPassword reads the password without echo, twice, when stdin is a terminal, and otherwise the
// first line of stdin. Prompts go to prompt (stderr), never the password itself. An empty password
// is refused: it would create an owner who cannot sign in.
func readPassword(stdin *os.File, prompt io.Writer) (string, error) {
	password, err := readRawPassword(stdin, prompt)
	if err == nil && password == "" {
		return "", errPasswordEmpty
	}
	return password, err
}

func readRawPassword(stdin *os.File, prompt io.Writer) (string, error) {
	fd := int(stdin.Fd())
	if !term.IsTerminal(fd) {
		return readPasswordLine(stdin)
	}
	read := func(label string) (string, error) {
		fmt.Fprint(prompt, label)
		b, err := term.ReadPassword(fd)
		fmt.Fprintln(prompt)
		if err != nil {
			return "", fmt.Errorf("read password: %w", err)
		}
		return string(b), nil
	}
	first, err := read("Owner password: ")
	if err != nil {
		return "", err
	}
	second, err := read("Repeat password: ")
	if err != nil {
		return "", err
	}
	if first != second {
		return "", errPasswordMismatch
	}
	return first, nil
}

// readPasswordLine returns the first line of r without its line ending.
func readPasswordLine(r io.Reader) (string, error) {
	line, err := bufio.NewReader(r).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read password: %w", err)
	}
	return strings.TrimRight(line, "\r\n"), nil
}
