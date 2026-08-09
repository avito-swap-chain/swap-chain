package httpserver

import (
	"net/http"
	"runtime/debug"
	"time"

	"go.uber.org/zap"
)

type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (r *responseRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *responseRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func accessLog(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			started := time.Now()
			recorder := &responseRecorder{ResponseWriter: writer, status: http.StatusOK}
			next.ServeHTTP(recorder, request)
			logger.Info("HTTP request",
				zap.String("method", request.Method),
				zap.String("path", request.URL.Path),
				zap.Int("status", recorder.status),
				zap.Duration("duration", time.Since(started)),
			)
		})
	}
}

func recoverJSON(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					logger.Error("HTTP handler panic", zap.Any("error", recovered), zap.ByteString("stack", debug.Stack()))
					writeError(writer, request, http.StatusInternalServerError, "INTERNAL_ERROR", "unexpected server error")
				}
			}()
			next.ServeHTTP(writer, request)
		})
	}
}

func cors(allowedOrigin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			origin := request.Header.Get("Origin")
			if origin != "" && origin == allowedOrigin {
				writer.Header().Set("Access-Control-Allow-Origin", origin)
				writer.Header().Set("Access-Control-Allow-Credentials", "true")
				writer.Header().Add("Vary", "Origin")
			}
			writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Last-Event-ID, X-Request-ID")
			writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			if request.Method == http.MethodOptions {
				writer.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(writer, request)
		})
	}
}

const defaultBodyLimit = 1 << 20

func limitBody(maxMediaUploadBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if request.Body != nil {
				limit := int64(defaultBodyLimit)
				if request.Method == http.MethodPost && request.URL.Path == "/api/v1/media" {
					limit = maxMediaUploadBytes + defaultBodyLimit
				}
				request.Body = http.MaxBytesReader(writer, request.Body, limit)
			}
			next.ServeHTTP(writer, request)
		})
	}
}
