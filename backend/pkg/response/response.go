package response

import (
	"encoding/json"
	"net/http"
)

// JSON writes a JSON response with the given HTTP status code and data payload.
func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// Error writes a standardised error response body conforming to the API spec:
//
//	{ "error": { "code": "<code>", "message": "<message>" } }
func Error(w http.ResponseWriter, status int, code, message string) {
	JSON(w, status, map[string]any{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}
