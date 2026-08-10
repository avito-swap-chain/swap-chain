// Package httpserver assembles middleware and the generated strict HTTP router.
package httpserver

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"

	"swap-chain/internal/api"
)

func writeError(writer http.ResponseWriter, request *http.Request, status int, code, message string) {
	requestID := middleware.GetReqID(request.Context())
	response := api.Error{Code: code, Message: message}
	if requestID != "" {
		response.RequestId = &requestID
	}

	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(response)
}
