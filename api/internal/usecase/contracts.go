// Package usecase holds the business rules, one sub-package per domain (ADR 0003).
//
// This file declares the use-case interfaces the controller layer depends on. Use cases import only
// the standard library, internal/entity and internal/repo contracts; lint rules enforce it.
// Tests for every domain live in this directory (package usecase_test) and use the generated mocks.
package usecase

//go:generate go tool -modfile=../../tools/go.mod mockgen -source=contracts.go -destination=./mocks_usecase_test.go -package=usecase_test

import "context"

type (
	// Mail sends the email outbox (ADR 0010). The worker in internal/app calls it.
	Mail interface {
		// SendDue claims due emails and sends them one at a time, outside any transaction, then marks
		// each sent or reschedules it. It returns how many were claimed; zero means none was due.
		// Once ctx ends it finishes the email in flight and leaves the rest to their lease.
		SendDue(ctx context.Context) (int, error)
		// Maintain clears the payloads of failed emails after 1 hour and purges sent emails after 30 days.
		Maintain(ctx context.Context) error
	}
)
