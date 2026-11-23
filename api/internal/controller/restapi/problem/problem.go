// Package problem writes RFC 9457 problem details, the error format of the whole API.
package problem

import (
	"encoding/json"
	"net/http"
)

// Problem is an RFC 9457 problem details body. Errors maps a request field to its first error message.
type Problem struct {
	Type   string            `json:"type"`
	Title  string            `json:"title"`
	Status int               `json:"status"`
	Detail string            `json:"detail,omitempty"`
	Errors map[string]string `json:"errors,omitempty"`
}

// New builds a problem for an HTTP status with the standard reason phrase as title.
func New(status int, detail string) Problem {
	return Problem{Type: "about:blank", Title: http.StatusText(status), Status: status, Detail: detail}
}

// Write sends p as application/problem+json.
func Write(w http.ResponseWriter, p Problem) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(p.Status)
	_ = json.NewEncoder(w).Encode(p)
}
