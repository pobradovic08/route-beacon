package model

import (
	"encoding/json"
	"net/http"
)

// ProblemDetail represents an RFC 7807 Problem Details response.
type ProblemDetail struct {
	Type          string         `json:"type"`
	Title         string         `json:"title"`
	Status        int            `json:"status"`
	Detail        string         `json:"detail,omitempty"`
	Instance      string         `json:"instance,omitempty"`
	InvalidParams []InvalidParam `json:"invalid_params,omitempty"`
}

// InvalidParam describes a single invalid request parameter.
type InvalidParam struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// NewProblem creates a ProblemDetail with the given status and detail message.
func NewProblem(status int, detail string) ProblemDetail {
	return ProblemDetail{
		Type:   "about:blank",
		Title:  http.StatusText(status),
		Status: status,
		Detail: detail,
	}
}

// WriteProblem writes an RFC 7807 error response.
func WriteProblem(w http.ResponseWriter, status int, detail string) {
	p := NewProblem(status, detail)
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(p)
}

// WriteProblemWithParams writes an RFC 7807 error with invalid parameter details.
func WriteProblemWithParams(w http.ResponseWriter, status int, detail string, params []InvalidParam) {
	p := NewProblem(status, detail)
	p.InvalidParams = params
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(p)
}
