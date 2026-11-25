// Package usecase holds the business rules, one sub-package per domain (ADR 0003).
//
// This file declares the use-case interfaces the controller layer depends on. Use cases import only
// the standard library, internal/entity and internal/repo contracts; lint rules enforce it.
// Tests for every domain live in this directory (package usecase_test) and use the generated mocks.
package usecase

//go:generate go tool -modfile=../../tools/go.mod mockgen -source=contracts.go -destination=./mocks_usecase_test.go -package=usecase_test
