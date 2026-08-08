package httptransport

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"swap-chain/matching/model"

	"go.uber.org/zap"
)

type pingerStub struct {
	err error
}

func (stub pingerStub) PingContext(context.Context) error {
	return stub.err
}

type finderStub struct {
	cycles [][]model.Edge
	err    error
	itemID int
}

func (stub *finderStub) Execute(_ context.Context, itemID int) ([][]model.Edge, error) {
	stub.itemID = itemID
	return stub.cycles, stub.err
}

func TestHealthReportsDatabaseState(t *testing.T) {
	tests := []struct {
		name       string
		pingError  error
		wantStatus int
		wantBody   string
	}{
		{name: "healthy", wantStatus: http.StatusOK, wantBody: `"database":"up"`},
		{name: "database unavailable", pingError: errors.New("down"), wantStatus: http.StatusServiceUnavailable, wantBody: `"code":"database_unavailable"`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := NewHandler(pingerStub{err: test.pingError}, &finderStub{}, zap.NewNop()).Routes()
			request := httptest.NewRequest(http.MethodGet, "/health", nil)
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
			if !strings.Contains(response.Body.String(), test.wantBody) {
				t.Fatalf("body = %q, want to contain %q", response.Body.String(), test.wantBody)
			}
		})
	}
}

func TestFindCyclesReturnsMatchingResponse(t *testing.T) {
	finder := &finderStub{cycles: [][]model.Edge{{{SourceID: 7, TargetID: 8, Score: 0.75}}}}
	handler := NewHandler(pingerStub{}, finder, zap.NewNop()).Routes()
	request := httptest.NewRequest(http.MethodGet, "/items/7/matching", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if finder.itemID != 7 {
		t.Fatalf("finder item ID = %d, want 7", finder.itemID)
	}
	if !strings.Contains(response.Body.String(), `"targetItemId":8`) {
		t.Fatalf("unexpected body: %s", response.Body.String())
	}
}

func TestFindCyclesRejectsInvalidItemID(t *testing.T) {
	handler := NewHandler(pingerStub{}, &finderStub{}, zap.NewNop()).Routes()
	request := httptest.NewRequest(http.MethodGet, "/items/nope/matching", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}
