package httpserver

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	nethttpmiddleware "github.com/oapi-codegen/nethttp-middleware"
	"go.uber.org/zap"

	"swap-chain/internal/api"
	"swap-chain/internal/session"
)

// New creates the validated HTTP router with demo-session and CORS middleware.
func New(logger *zap.Logger, handler api.StrictServerInterface, sessions *session.Manager, allowedOrigin string, maxMediaUploadBytes int64) (http.Handler, error) {
	specification, err := api.GetSpec()
	if err != nil {
		return nil, err
	}

	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(recoverJSON(logger))
	router.Use(accessLog(logger))
	router.Use(cors(allowedOrigin))
	router.Use(limitBody(maxMediaUploadBytes))
	router.Use(sessions.Middleware)
	router.Use(nethttpmiddleware.OapiRequestValidatorWithOptions(specification, &nethttpmiddleware.Options{
		DoNotValidateServers: true,
		Skipper: func(request *http.Request) bool {
			return request.Method == http.MethodOptions
		},
		ErrorHandlerWithOpts: func(_ context.Context, err error, writer http.ResponseWriter, request *http.Request, options nethttpmiddleware.ErrorHandlerOpts) {
			status := options.StatusCode
			if status == 0 {
				status = http.StatusBadRequest
			}
			writeError(writer, request, status, "INVALID_REQUEST", err.Error())
		},
	}))

	strictHandler := api.NewStrictHandlerWithOptions(handler, nil, api.StrictHTTPServerOptions{
		RequestErrorHandlerFunc: func(writer http.ResponseWriter, request *http.Request, err error) {
			writeError(writer, request, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		},
		ResponseErrorHandlerFunc: func(writer http.ResponseWriter, request *http.Request, err error) {
			logger.Error("failed to write HTTP response", zap.Error(err))
			writeError(writer, request, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to write response")
		},
	})

	router.NotFound(func(writer http.ResponseWriter, request *http.Request) {
		writeError(writer, request, http.StatusNotFound, "ROUTE_NOT_FOUND", "route not found")
	})
	router.MethodNotAllowed(func(writer http.ResponseWriter, request *http.Request) {
		writeError(writer, request, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
	})

	return api.HandlerFromMux(strictHandler, router), nil
}
