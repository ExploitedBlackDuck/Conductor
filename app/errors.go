package app

import (
	"errors"

	"github.com/conductor-app/conductor/internal/core/coreerr"
)

// ErrorDTO is the typed error shape crossing the Wails boundary (§2.2). The
// frontend switches on Code (drawn from the §8.4 catalog) and never parses
// Message. A raw Go error string never reaches the frontend.
type ErrorDTO struct {
	// Code is the stable catalog identifier.
	Code string `json:"code"`
	// Message is a human-readable, secret-free summary.
	Message string `json:"message"`
	// Retryable indicates whether retrying may succeed.
	Retryable bool `json:"retryable"`
}

// errorToDTO maps any error to an ErrorDTO. Coded errors carry their catalog
// code through; uncoded errors collapse to a generic internal code so the
// frontend always receives a well-formed, message-safe DTO.
func errorToDTO(err error) *ErrorDTO {
	if err == nil {
		return nil
	}
	var ce *coreerr.Error
	if errors.As(err, &ce) {
		return &ErrorDTO{
			Code:      string(ce.Code),
			Message:   ce.Message,
			Retryable: ce.Retryable,
		}
	}
	return &ErrorDTO{
		Code:      "ERR_INTERNAL",
		Message:   "an unexpected error occurred",
		Retryable: false,
	}
}
