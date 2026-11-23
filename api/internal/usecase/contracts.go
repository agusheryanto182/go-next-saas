// Package usecase holds the business rules, one sub-package per domain (ADR 0003).
//
// This file declares the use-case interfaces the controller layer depends on. Use cases import only
// the standard library, internal/entity and internal/repo contracts; lint rules enforce it.
package usecase
