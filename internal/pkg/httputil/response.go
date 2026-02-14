// Package httputil provides HTTP response helper functions.
package httputil

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-playground/validator/v10"
)

// JSON writes a raw JSON response without envelope.
// Use Success for {"data": ...} wrapped responses.
func JSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			slog.Error("failed to encode response", "error", err)
		}
	}
}

// Text writes a plain text response.
func Text(w http.ResponseWriter, statusCode int, text string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(statusCode)
	if _, err := w.Write([]byte(text)); err != nil {
		slog.Error("failed to write response", "error", err)
	}
}

// Success writes a JSON response with {"data": ...} envelope.
func Success(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]interface{}{"data": data}); err != nil {
		slog.Error("failed to encode response", "error", err)
	}
}

// Error writes a JSON response with {"error": {"message": ...}} envelope.
func Error(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{"message": message},
	}); err != nil {
		slog.Error("failed to encode error response", "error", err)
	}
}

// ValidationError writes a validation error response.
// If err is validator.ValidationErrors, returns structured field details.
// Otherwise, returns err.Error() as details string.
func ValidationError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)

	var details interface{}
	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		fieldErrors := make([]map[string]string, 0, len(validationErrors))
		for _, e := range validationErrors {
			fieldErrors = append(fieldErrors, map[string]string{
				"field":   e.Field(),
				"message": e.Tag(),
			})
		}
		details = fieldErrors
	} else {
		details = err.Error()
	}

	if encErr := json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"message": "validation error",
			"details": details,
		},
	}); encErr != nil {
		slog.Error("failed to encode validation error response", "error", encErr)
	}
}
