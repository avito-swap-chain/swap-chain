// Package httptransport exposes application use cases over HTTP.
package httptransport

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	applicationmatching "swap-chain/internal/application/matching"
	"swap-chain/matching/model"

	"go.uber.org/zap"
)

type databasePinger interface {
	PingContext(ctx context.Context) error
}

type cycleFinder interface {
	Execute(ctx context.Context, itemID int) ([][]model.Edge, error)
}

// Handler owns the public backend routes.
type Handler struct {
	database databasePinger
	finder   cycleFinder
	logger   *zap.Logger
}

// NewHandler constructs an HTTP handler with explicit database and matching dependencies.
func NewHandler(database databasePinger, finder cycleFinder, logger *zap.Logger) *Handler {
	return &Handler{database: database, finder: finder, logger: logger}
}

// Routes returns the fully configured HTTP routing tree.
func (handler *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handler.health)
	mux.HandleFunc("GET /items/{itemID}/matching", handler.findCycles)
	return mux
}

func (handler *Handler) health(response http.ResponseWriter, request *http.Request) {
	if err := handler.database.PingContext(request.Context()); err != nil {
		handler.logger.Error("database health check failed", zap.Error(err))
		handler.writeJSON(response, http.StatusServiceUnavailable, errorResponse{Code: "database_unavailable", Message: "database is unavailable"})
		return
	}

	handler.writeJSON(response, http.StatusOK, healthResponse{Status: "ok", Database: "up"})
}

func (handler *Handler) findCycles(response http.ResponseWriter, request *http.Request) {
	itemID, err := strconv.Atoi(request.PathValue("itemID"))
	if err != nil || itemID <= 0 {
		handler.writeJSON(response, http.StatusBadRequest, errorResponse{Code: "invalid_item_id", Message: "itemID must be a positive integer"})
		return
	}

	cycles, err := handler.finder.Execute(request.Context(), itemID)
	if err != nil {
		if errors.Is(err, applicationmatching.ErrInvalidItemID) {
			handler.writeJSON(response, http.StatusBadRequest, errorResponse{Code: "invalid_item_id", Message: err.Error()})
			return
		}

		handler.logger.Error("find matching cycles", zap.Int("item_id", itemID), zap.Error(err))
		handler.writeJSON(response, http.StatusInternalServerError, errorResponse{Code: "matching_failed", Message: "matching could not be completed"})
		return
	}

	responseCycles := make([]cycleResponse, 0, len(cycles))
	for _, cycle := range cycles {
		edges := make([]edgeResponse, 0, len(cycle))
		for _, edge := range cycle {
			edges = append(edges, edgeResponse{
				SourceItemID: edge.SourceID,
				TargetItemID: edge.TargetID,
				Score:        edge.Score,
			})
		}
		responseCycles = append(responseCycles, cycleResponse{Edges: edges})
	}

	handler.writeJSON(response, http.StatusOK, matchingResponse{ItemID: itemID, Cycles: responseCycles})
}

func (handler *Handler) writeJSON(response http.ResponseWriter, status int, payload any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	if err := json.NewEncoder(response).Encode(payload); err != nil {
		handler.logger.Error("encode HTTP response", zap.Error(err))
	}
}

type healthResponse struct {
	Status   string `json:"status"`
	Database string `json:"database"`
}

type matchingResponse struct {
	ItemID int             `json:"itemId"`
	Cycles []cycleResponse `json:"cycles"`
}

type cycleResponse struct {
	Edges []edgeResponse `json:"edges"`
}

type edgeResponse struct {
	SourceItemID int     `json:"sourceItemId"`
	TargetItemID int     `json:"targetItemId"`
	Score        float64 `json:"score"`
}

type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
