// Package entity holds the business models and domain errors shared by every layer (ADR 0003).
// Entities depend on nothing but the standard library; IDs are UUID strings, parsed and formatted at
// the storage boundary.
package entity
