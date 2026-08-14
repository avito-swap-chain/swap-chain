package httpserver

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"

	"swap-chain/internal/api"
	"swap-chain/internal/categories"
	"swap-chain/internal/chains"
	"swap-chain/internal/events"
	"swap-chain/internal/httpapi"
	"swap-chain/internal/items"
	"swap-chain/internal/media"
	"swap-chain/internal/session"
	"swap-chain/internal/users"
	adminmodel "swap-chain/modules/admin/model"
	analyzemodel "swap-chain/modules/analyze/model"
	blocklistmodel "swap-chain/modules/blocklist/model"
	chatmodel "swap-chain/modules/chat/model"
	"swap-chain/modules/matching/model"
	metricsmodel "swap-chain/modules/metrics/model"
	moderationmodel "swap-chain/modules/moderation/model"
	notificationmodel "swap-chain/modules/notifications/model"
	notificationservice "swap-chain/modules/notifications/service"
	reputationmodel "swap-chain/modules/reputation/model"
)

func TestHealthAndMatchingRoutesUseIntegratedServices(t *testing.T) {
	itemService := matchingReadyItems{Service: items.NewMemoryService()}
	server := newTestServerWithServices(t, &testChainService{chain: testChain()}, itemService)
	defer server.Close()

	for _, path := range []string{"/health", "/api/v1/health"} {
		response, err := http.Get(server.URL + path) //nolint:bodyclose // Closed on every branch below.
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		if response.StatusCode != http.StatusOK {
			body := readBody(t, response.Body)
			closeBody(t, response.Body)
			t.Fatalf("GET %s status = %d, want %d; body=%s", path, response.StatusCode, http.StatusOK, body)
		}
		closeBody(t, response.Body)
	}

	client := newSessionClient(t, server.URL, 1)
	createdResponse := postJSON(t, client, server.URL+"/api/v1/items", validItemPayload) //nolint:bodyclose // Closed on every branch below.
	if createdResponse.StatusCode != http.StatusCreated {
		body := readBody(t, createdResponse.Body)
		closeBody(t, createdResponse.Body)
		t.Fatalf("create matching root status = %d, want %d; body=%s", createdResponse.StatusCode, http.StatusCreated, body)
	}
	var created api.Item
	if err := json.NewDecoder(createdResponse.Body).Decode(&created); err != nil {
		closeBody(t, createdResponse.Body)
		t.Fatalf("decode created matching root: %v", err)
	}
	closeBody(t, createdResponse.Body)

	response, err := client.Get(fmt.Sprintf("%s/api/v1/items/%d/matching", server.URL, created.Id))
	if err != nil {
		t.Fatalf("GET matching: %v", err)
	}
	defer closeBody(t, response.Body)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("matching status = %d, want %d", response.StatusCode, http.StatusOK)
	}

	var body api.MatchingResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode matching: %v", err)
	}
	if len(body.Cycles) != 1 || len(body.Cycles[0].Edges) != 2 {
		t.Fatalf("unexpected matching response: %#v", body)
	}
}

func TestLivenessDoesNotDependOnReadiness(t *testing.T) {
	logger := zap.NewNop()
	eventHub := events.NewHub()
	sessions := session.NewManager(time.Hour, false)
	handler := httpapi.NewHandler(
		testDatabase{},
		testFinder{},
		logger,
		items.NewMemoryService(),
		media.NewService(newTestMediaStorage(), 10<<20),
		&testChainService{chain: testChain()},
		&testCategoriesService{},
		eventHub,
		sessions,
		users.NewMemoryService(testUser(1)),
		&testAdminService{},
		newTestChatService(),
		&testBlocklistService{},
		&testModerationService{},
		nil,
		&testReputationService{},
		&testMetricsService{},
		nil,
		testReadiness{err: errors.New("ollama model is missing")},
	)
	router, err := New(logger, handler, sessions, "http://localhost:5173", 10<<20)
	if err != nil {
		t.Fatalf("create HTTP handler: %v", err)
	}
	server := httptest.NewServer(router)
	defer server.Close()

	liveness, err := http.Get(server.URL + "/health")
	if err != nil {
		t.Fatalf("GET liveness: %v", err)
	}
	defer closeBody(t, liveness.Body)
	if liveness.StatusCode != http.StatusOK {
		t.Fatalf("liveness status = %d, want 200", liveness.StatusCode)
	}

	readiness, err := http.Get(server.URL + "/api/v1/health")
	if err != nil {
		t.Fatalf("GET readiness: %v", err)
	}
	defer closeBody(t, readiness.Body)
	if readiness.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("readiness status = %d, want 503", readiness.StatusCode)
	}
}

func TestMatchingRejectsItemBeforeAnalysis(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	client := newSessionClient(t, server.URL, 1)
	createdResponse := postJSON(t, client, server.URL+"/api/v1/items", validItemPayload)
	if createdResponse.StatusCode != http.StatusCreated {
		defer closeBody(t, createdResponse.Body)
		t.Fatalf("create item status = %d, want %d", createdResponse.StatusCode, http.StatusCreated)
	}
	var created api.Item
	if err := json.NewDecoder(createdResponse.Body).Decode(&created); err != nil {
		closeBody(t, createdResponse.Body)
		t.Fatalf("decode created item: %v", err)
	}
	closeBody(t, createdResponse.Body)

	response, err := client.Get(fmt.Sprintf("%s/api/v1/items/%d/matching", server.URL, created.Id))
	if err != nil {
		t.Fatalf("GET early matching: %v", err)
	}
	defer closeBody(t, response.Body)
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("early matching status = %d, want %d", response.StatusCode, http.StatusConflict)
	}
}

func TestMatchingRejectsForeignItem(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	owner := newSessionClient(t, server.URL, 1)
	createdResponse := postJSON(t, owner, server.URL+"/api/v1/items", validItemPayload)
	if createdResponse.StatusCode != http.StatusCreated {
		defer closeBody(t, createdResponse.Body)
		t.Fatalf("create item status = %d, want %d", createdResponse.StatusCode, http.StatusCreated)
	}
	var created api.Item
	if err := json.NewDecoder(createdResponse.Body).Decode(&created); err != nil {
		closeBody(t, createdResponse.Body)
		t.Fatalf("decode created item: %v", err)
	}
	closeBody(t, createdResponse.Body)

	other := newSessionClient(t, server.URL, 2)
	response, err := other.Get(fmt.Sprintf("%s/api/v1/items/%d/matching", server.URL, created.Id))
	if err != nil {
		t.Fatalf("GET foreign matching: %v", err)
	}
	defer closeBody(t, response.Body)
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("foreign matching status = %d, want %d", response.StatusCode, http.StatusForbidden)
	}
}

func TestSessionIsRequiredForPersonalRoutes(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	for _, test := range []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodGet, path: "/api/v1/events"},
		{method: http.MethodGet, path: "/api/v1/chains"},
		{method: http.MethodGet, path: "/api/v1/items"},
		{method: http.MethodGet, path: "/api/v1/items/1/matching"},
		{method: http.MethodPost, path: "/api/v1/chains", body: validChainPayload},
		{method: http.MethodPost, path: "/api/v1/chains/12/receipt"},
		{method: http.MethodGet, path: "/api/v1/chat/threads"},
		{method: http.MethodGet, path: "/api/v1/items/202/chat/2/messages"},
		{method: http.MethodPost, path: "/api/v1/items/202/chat/2/messages", body: validChatPayload},
		{method: http.MethodPost, path: "/api/v1/items/202/chat/2/read", body: `{"lastReadMessageId":1}`},
		{method: http.MethodPost, path: "/api/v1/items", body: validItemPayload},
		{method: http.MethodGet, path: "/api/v1/admin/deliveries"},
		{method: http.MethodGet, path: "/api/v1/admin/metrics/funnel"},
		{method: http.MethodPost, path: "/api/v1/admin/deliveries/1/transition", body: `{"status":"AT_PVZ"}`},
	} {
		request, err := http.NewRequest(test.method, server.URL+test.path, strings.NewReader(test.body))
		if err != nil {
			t.Fatalf("create request: %v", err)
		}
		if test.body != "" {
			request.Header.Set("Content-Type", "application/json")
		}

		response, err := http.DefaultClient.Do(request) //nolint:bodyclose // Closed on every branch below.
		if err != nil {
			t.Fatalf("%s %s: %v", test.method, test.path, err)
		}
		if response.StatusCode != http.StatusUnauthorized {
			body := readBody(t, response.Body)
			closeBody(t, response.Body)
			t.Fatalf("%s %s status = %d, want %d; body=%s", test.method, test.path, response.StatusCode, http.StatusUnauthorized, body)
		}
		closeBody(t, response.Body)
	}
}

func TestChatContractUsesSessionActorAndMapsLifecycle(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	participant := newSessionClient(t, server.URL, 1)
	created := postJSON(t, participant, server.URL+"/api/v1/items/202/chat/2/messages", validChatPayload)
	defer closeBody(t, created.Body)
	if created.StatusCode != http.StatusCreated {
		body := readBody(t, created.Body)
		t.Fatalf("send status = %d, want %d; body=%s", created.StatusCode, http.StatusCreated, body)
	}
	var sent api.ChatMessage
	if err := json.NewDecoder(created.Body).Decode(&sent); err != nil {
		t.Fatalf("decode sent message: %v", err)
	}
	if sent.Sender.Id != 1 || sent.Recipient.Id != 2 || sent.Text != "Привет участникам!" {
		t.Fatalf("sent message = %+v", sent)
	}

	repeated := postJSON(t, participant, server.URL+"/api/v1/items/202/chat/2/messages", validChatPayload)
	defer closeBody(t, repeated.Body)
	if repeated.StatusCode != http.StatusOK {
		t.Fatalf("idempotent retry status = %d, want %d", repeated.StatusCode, http.StatusOK)
	}

	listed, err := participant.Get(server.URL + "/api/v1/items/202/chat/2/messages?afterId=0&limit=10&waitSeconds=0")
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	defer closeBody(t, listed.Body)
	if listed.StatusCode != http.StatusOK {
		body := readBody(t, listed.Body)
		t.Fatalf("list status = %d, want %d; body=%s", listed.StatusCode, http.StatusOK, body)
	}
	var messages api.ChatMessageList
	if err := json.NewDecoder(listed.Body).Decode(&messages); err != nil {
		t.Fatalf("decode messages: %v", err)
	}
	if len(messages.Messages) != 1 || messages.NextAfterId == nil || *messages.NextAfterId != sent.Id {
		t.Fatalf("messages = %+v", messages)
	}

	counterpart := newSessionClient(t, server.URL, 2)
	received := postJSON(t, counterpart, server.URL+"/api/v1/items/202/chat/1/messages", validChatPayload)
	defer closeBody(t, received.Body)
	if received.StatusCode != http.StatusCreated {
		body := readBody(t, received.Body)
		t.Fatalf("counterpart send status = %d, want %d; body=%s", received.StatusCode, http.StatusCreated, body)
	}
	var incoming api.ChatMessage
	if err := json.NewDecoder(received.Body).Decode(&incoming); err != nil {
		t.Fatalf("decode counterpart message: %v", err)
	}

	threadsResponse, err := participant.Get(server.URL + "/api/v1/chat/threads")
	if err != nil {
		t.Fatalf("list chat threads: %v", err)
	}
	defer closeBody(t, threadsResponse.Body)
	var threads api.ChatThreadList
	if err := json.NewDecoder(threadsResponse.Body).Decode(&threads); err != nil {
		t.Fatalf("decode chat threads: %v", err)
	}
	if threadsResponse.StatusCode != http.StatusOK || len(threads.Threads) != 1 || threads.Threads[0].Counterpart.Id != 2 ||
		threads.Threads[0].Item.Id != 202 || threads.TotalUnreadCount != 1 {
		t.Fatalf("threads status=%d body=%+v", threadsResponse.StatusCode, threads)
	}

	marked := postJSON(t, participant, server.URL+"/api/v1/items/202/chat/2/read", fmt.Sprintf(`{"lastReadMessageId":%d}`, incoming.Id))
	defer closeBody(t, marked.Body)
	var readState api.ChatReadState
	if err := json.NewDecoder(marked.Body).Decode(&readState); err != nil {
		t.Fatalf("decode read state: %v", err)
	}
	if marked.StatusCode != http.StatusOK || readState.UnreadCount != 0 || readState.LastReadMessageId != incoming.Id {
		t.Fatalf("read state status=%d body=%+v", marked.StatusCode, readState)
	}

	for _, test := range []struct {
		name   string
		client *http.Client
		path   string
		want   int
	}{
		{name: "another shared item", client: participant, path: "/api/v1/items/203/chat/2/messages", want: http.StatusOK},
		{name: "missing item", client: participant, path: "/api/v1/items/404/chat/2/messages", want: http.StatusNotFound},
		{name: "unknown counterpart", client: participant, path: "/api/v1/items/202/chat/7/messages", want: http.StatusNotFound},
		{name: "outsider", client: newSessionClient(t, server.URL, 7), path: "/api/v1/items/202/chat/2/messages", want: http.StatusForbidden},
	} {
		t.Run(test.name, func(t *testing.T) {
			response, err := test.client.Get(server.URL + test.path)
			if err != nil {
				t.Fatalf("GET chat: %v", err)
			}
			defer closeBody(t, response.Body)
			if response.StatusCode != test.want {
				t.Fatalf("status = %d, want %d", response.StatusCode, test.want)
			}
		})
	}
}

func TestAdminDeliveryContractEnforcesRoleAndMapsTransitions(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	regularUser := newSessionClient(t, server.URL, 1)
	forbidden, err := regularUser.Get(server.URL + "/api/v1/admin/deliveries")
	if err != nil {
		t.Fatalf("list as regular user: %v", err)
	}
	defer closeBody(t, forbidden.Body)
	if forbidden.StatusCode != http.StatusForbidden {
		t.Fatalf("regular user status = %d, want %d", forbidden.StatusCode, http.StatusForbidden)
	}

	admin := newSessionClient(t, server.URL, 7)
	listed, err := admin.Get(server.URL + "/api/v1/admin/deliveries?status=AWAITING_PVZ")
	if err != nil {
		t.Fatalf("list as admin: %v", err)
	}
	defer closeBody(t, listed.Body)
	if listed.StatusCode != http.StatusOK {
		body := readBody(t, listed.Body)
		t.Fatalf("admin list status = %d, want %d; body=%s", listed.StatusCode, http.StatusOK, body)
	}
	var list api.AdminDeliveryList
	if err := json.NewDecoder(listed.Body).Decode(&list); err != nil {
		t.Fatalf("decode admin list: %v", err)
	}
	if len(list.Deliveries) != 1 || list.Deliveries[0].Status != api.AdminDeliveryStatusAWAITINGPVZ {
		t.Fatalf("admin deliveries = %#v", list.Deliveries)
	}

	transitioned := postJSON(t, admin, server.URL+"/api/v1/admin/deliveries/41/transition", `{"status":"AT_PVZ"}`)
	defer closeBody(t, transitioned.Body)
	if transitioned.StatusCode != http.StatusOK {
		t.Fatalf("transition status = %d, want %d; body=%s", transitioned.StatusCode, http.StatusOK, readBody(t, transitioned.Body))
	}

	received := postJSON(t, admin, server.URL+"/api/v1/admin/deliveries/41/transition", `{"status":"RECEIVED"}`)
	defer closeBody(t, received.Body)
	if received.StatusCode != http.StatusOK {
		t.Fatalf("receipt status = %d, want %d; body=%s", received.StatusCode, http.StatusOK, readBody(t, received.Body))
	}

	missing := postJSON(t, admin, server.URL+"/api/v1/admin/deliveries/404/transition", `{"status":"AT_PVZ"}`)
	defer closeBody(t, missing.Body)
	if missing.StatusCode != http.StatusNotFound {
		t.Fatalf("missing status = %d, want %d", missing.StatusCode, http.StatusNotFound)
	}

	conflict := postJSON(t, admin, server.URL+"/api/v1/admin/deliveries/42/transition", `{"status":"IN_DELIVERY"}`)
	defer closeBody(t, conflict.Body)
	if conflict.StatusCode != http.StatusConflict {
		t.Fatalf("conflict status = %d, want %d", conflict.StatusCode, http.StatusConflict)
	}
}

func TestAdminFunnelMetricsContractEnforcesRoleAndMapsSnapshot(t *testing.T) {
	server := newTestServerWithServices(t, &testChainService{chain: testChain()}, items.NewMemoryService())
	defer server.Close()

	regularUser := newSessionClient(t, server.URL, 1)
	forbidden, err := regularUser.Get(server.URL + "/api/v1/admin/metrics/funnel")
	if err != nil {
		t.Fatalf("get metrics as regular user: %v", err)
	}
	defer closeBody(t, forbidden.Body)
	if forbidden.StatusCode != http.StatusForbidden {
		body := readBody(t, forbidden.Body)
		t.Fatalf("regular user metrics status = %d, want %d; body=%s", forbidden.StatusCode, http.StatusForbidden, body)
	}

	admin := newSessionClient(t, server.URL, 7)
	response, err := admin.Get(server.URL + "/api/v1/admin/metrics/funnel")
	if err != nil {
		t.Fatalf("get metrics as admin: %v", err)
	}
	defer closeBody(t, response.Body)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("admin metrics status = %d, want %d; body=%s", response.StatusCode, http.StatusOK, readBody(t, response.Body))
	}
	var snapshot api.FunnelMetrics
	if err := json.NewDecoder(response.Body).Decode(&snapshot); err != nil {
		t.Fatalf("decode metrics: %v", err)
	}
	if snapshot.EligibleItems != 10 || snapshot.ItemsWithChain != 5 || snapshot.AcceptedChains != 2 || snapshot.CompletedChains != 1 {
		t.Fatalf("funnel metrics = %#v", snapshot)
	}
	if snapshot.ItemsWithChainRate == nil || *snapshot.ItemsWithChainRate != 0.5 || len(snapshot.RejectionReasons) != 1 {
		t.Fatalf("funnel rates/reasons = %#v", snapshot)
	}
}

func TestChainReceiptContractUsesSessionRecipient(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	recipient := newSessionClient(t, server.URL, 2)
	received := postJSON(t, recipient, server.URL+"/api/v1/chains/12/receipt", "")
	defer closeBody(t, received.Body)
	if received.StatusCode != http.StatusOK {
		body := readBody(t, received.Body)
		t.Fatalf("receipt status = %d, want %d; body=%s", received.StatusCode, http.StatusOK, body)
	}
	var receipt api.ChainReceipt
	if err := json.NewDecoder(received.Body).Decode(&receipt); err != nil {
		t.Fatalf("decode receipt: %v", err)
	}
	if receipt.Delivery.Status != api.AdminDeliveryStatusRECEIVED || receipt.ChainStatus != api.COMPLETED {
		t.Fatalf("receipt = %+v", receipt)
	}

	for _, test := range []struct {
		name    string
		userID  int64
		chainID int64
		want    int
	}{
		{name: "sender or outsider", userID: 1, chainID: 12, want: http.StatusForbidden},
		{name: "wrong chain state", userID: 2, chainID: 43, want: http.StatusConflict},
		{name: "missing chain", userID: 2, chainID: 404, want: http.StatusNotFound},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := newSessionClient(t, server.URL, test.userID)
			response := postJSON(t, client, fmt.Sprintf("%s/api/v1/chains/%d/receipt", server.URL, test.chainID), "")
			defer closeBody(t, response.Body)
			if response.StatusCode != test.want {
				t.Fatalf("status = %d, want %d; body=%s", response.StatusCode, test.want, readBody(t, response.Body))
			}
		})
	}
}

func TestLoginCookieIdentifiesCurrentUser(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	client := newSessionClient(t, server.URL, 7)
	response, err := client.Get(server.URL + "/api/v1/session")
	if err != nil {
		t.Fatalf("GET session: %v", err)
	}
	defer closeBody(t, response.Body)

	var current api.Session
	if err := json.NewDecoder(response.Body).Decode(&current); err != nil {
		t.Fatalf("decode session: %v", err)
	}
	if current.User.Id != 7 || current.User.Phone != phoneForUserID(7) || current.User.Role != api.ADMIN {
		t.Fatalf("current user = %#v, want user 7", current.User)
	}
}

func TestRegistrationUnknownLoginAndLogout(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("create cookie jar: %v", err)
	}
	client := &http.Client{Jar: jar}

	missing := postJSON(t, client, server.URL+"/api/v1/session", `{"phone":"+7 999 555-00-00"}`) //nolint:bodyclose // Closed below.
	if missing.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown login status = %d, want %d; body=%s", missing.StatusCode, http.StatusNotFound, readBody(t, missing.Body))
	}
	closeBody(t, missing.Body)

	registered := postJSON(t, client, server.URL+"/api/v1/users", `{"username":"  Danya  ","phone":"8 (999) 555-00-00"}`) //nolint:bodyclose // Closed below.
	if registered.StatusCode != http.StatusCreated {
		t.Fatalf("registration status = %d, want %d; body=%s", registered.StatusCode, http.StatusCreated, readBody(t, registered.Body))
	}
	var current api.Session
	if err := json.NewDecoder(registered.Body).Decode(&current); err != nil {
		t.Fatalf("decode registration: %v", err)
	}
	closeBody(t, registered.Body)
	if current.User.Username != "Danya" || current.User.Phone != "+79995550000" || current.User.Role != api.USER {
		t.Fatalf("registered user = %#v", current.User)
	}

	duplicate := postJSON(t, http.DefaultClient, server.URL+"/api/v1/users", `{"username":"Other","phone":"+7 999 555-00-00"}`) //nolint:bodyclose // Closed below.
	if duplicate.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate status = %d, want %d; body=%s", duplicate.StatusCode, http.StatusConflict, readBody(t, duplicate.Body))
	}
	closeBody(t, duplicate.Body)

	logoutRequest, err := http.NewRequest(http.MethodDelete, server.URL+"/api/v1/session", nil)
	if err != nil {
		t.Fatalf("create logout request: %v", err)
	}
	logout, err := client.Do(logoutRequest) //nolint:bodyclose // Closed below.
	if err != nil {
		t.Fatalf("logout: %v", err)
	}
	closeBody(t, logout.Body)
	if logout.StatusCode != http.StatusNoContent {
		t.Fatalf("logout status = %d, want %d", logout.StatusCode, http.StatusNoContent)
	}

	afterLogout, err := client.Get(server.URL + "/api/v1/session")
	if err != nil {
		t.Fatalf("get session after logout: %v", err)
	}
	defer closeBody(t, afterLogout.Body)
	if afterLogout.StatusCode != http.StatusUnauthorized {
		t.Fatalf("session after logout status = %d, want %d", afterLogout.StatusCode, http.StatusUnauthorized)
	}
}

func TestGetUserProfile(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	response, err := http.Get(fmt.Sprintf("%s/api/v1/users/1", server.URL))
	if err != nil {
		t.Fatalf("GET user profile: %v", err)
	}
	defer closeBody(t, response.Body)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", response.StatusCode, http.StatusOK, readBody(t, response.Body))
	}
	var profile api.UserProfile
	if err := json.NewDecoder(response.Body).Decode(&profile); err != nil {
		t.Fatalf("decode user profile: %v", err)
	}
	if profile.Id != 1 || profile.Username != "user-1" {
		t.Fatalf("user profile = %#v, want id=1 username=user-1", profile)
	}
	if profile.Rating == nil || *profile.Rating != 4.5 || profile.ReviewsCount != 3 || profile.CompletedExchanges != 2 {
		t.Fatalf("user reputation = %#v", profile)
	}
}

func TestCreateAndListUserReviews(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()
	client := newSessionClient(t, server.URL, 1)

	created := postJSON(t, client, server.URL+"/api/v1/chains/42/reviews", `{"targetUserId":2,"rating":5,"text":"Отличный обмен"}`)
	defer closeBody(t, created.Body)
	if created.StatusCode != http.StatusCreated {
		t.Fatalf("create review status = %d, want 201; body=%s", created.StatusCode, readBody(t, created.Body))
	}
	var review api.UserReview
	if err := json.NewDecoder(created.Body).Decode(&review); err != nil {
		t.Fatalf("decode review: %v", err)
	}
	if review.ChainId != 42 || review.TargetUserId != 2 || review.Rating != 5 || review.Author.Id != 1 {
		t.Fatalf("created review = %#v", review)
	}

	listed, err := http.Get(server.URL + "/api/v1/users/2/reviews")
	if err != nil {
		t.Fatalf("GET user reviews: %v", err)
	}
	defer closeBody(t, listed.Body)
	if listed.StatusCode != http.StatusOK {
		t.Fatalf("list reviews status = %d, want 200", listed.StatusCode)
	}
}

func TestGetUserProfileNotFound(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	response, err := http.Get(fmt.Sprintf("%s/api/v1/users/999", server.URL))
	if err != nil {
		t.Fatalf("GET missing user: %v", err)
	}
	defer closeBody(t, response.Body)
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", response.StatusCode, http.StatusNotFound, readBody(t, response.Body))
	}
}

func TestUpdateUserProfile(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	client := newSessionClient(t, server.URL, 1)
	response := patchJSON(t, client, fmt.Sprintf("%s/api/v1/users/1", server.URL), `{"username":"  New Name  "}`)
	defer closeBody(t, response.Body)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", response.StatusCode, http.StatusOK, readBody(t, response.Body))
	}
	var profile api.UserProfile
	if err := json.NewDecoder(response.Body).Decode(&profile); err != nil {
		t.Fatalf("decode updated profile: %v", err)
	}
	if profile.Username != "New Name" || profile.Id != 1 {
		t.Fatalf("updated profile = %#v, want username=New Name", profile)
	}
}

func TestUpdateUserProfileRejectsForeignUser(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	client := newSessionClient(t, server.URL, 1)
	response := patchJSON(t, client, fmt.Sprintf("%s/api/v1/users/2", server.URL), `{"username":"Hijack"}`)
	defer closeBody(t, response.Body)
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", response.StatusCode, http.StatusForbidden, readBody(t, response.Body))
	}
}

func TestUpdateUserProfileRequiresSession(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	response := patchJSON(t, http.DefaultClient, fmt.Sprintf("%s/api/v1/users/1", server.URL), `{"username":"NoSession"}`)
	defer closeBody(t, response.Body)
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; body=%s", response.StatusCode, http.StatusUnauthorized, readBody(t, response.Body))
	}
}

func TestUpdateUserProfileValidatesInput(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	client := newSessionClient(t, server.URL, 1)
	response := patchJSON(t, client, fmt.Sprintf("%s/api/v1/users/1", server.URL), `{"username":"  "}`)
	defer closeBody(t, response.Body)
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", response.StatusCode, http.StatusBadRequest, readBody(t, response.Body))
	}
}

func TestUpdateUserProfileAvatar(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	client := newSessionClient(t, server.URL, 1)
	const avatarURL = "/api/v1/media/test-avatar.png"
	response := patchJSON(t, client, fmt.Sprintf("%s/api/v1/users/1", server.URL), fmt.Sprintf(`{"avatarUrl":%q}`, avatarURL))
	defer closeBody(t, response.Body)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", response.StatusCode, http.StatusOK, readBody(t, response.Body))
	}
	var profile api.UserProfile
	if err := json.NewDecoder(response.Body).Decode(&profile); err != nil {
		t.Fatalf("decode profile: %v", err)
	}
	if profile.AvatarUrl == nil || *profile.AvatarUrl != avatarURL {
		t.Fatalf("avatarUrl = %v, want %q", profile.AvatarUrl, avatarURL)
	}
	if profile.Username != "user-1" {
		t.Fatalf("username changed unexpectedly: %s", profile.Username)
	}
}

func TestUpdateUserProfileRemoveAvatar(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	client := newSessionClient(t, server.URL, 1)
	setResponse := patchJSON(t, client, fmt.Sprintf("%s/api/v1/users/1", server.URL), `{"avatarUrl":"/api/v1/media/test-avatar.png"}`)
	defer closeBody(t, setResponse.Body)

	removeResponse := patchJSON(t, client, fmt.Sprintf("%s/api/v1/users/1", server.URL), `{"avatarUrl":""}`)
	defer closeBody(t, removeResponse.Body)
	if removeResponse.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", removeResponse.StatusCode, http.StatusOK, readBody(t, removeResponse.Body))
	}
	var profile api.UserProfile
	if err := json.NewDecoder(removeResponse.Body).Decode(&profile); err != nil {
		t.Fatalf("decode profile: %v", err)
	}
	if profile.AvatarUrl != nil {
		t.Fatalf("avatarUrl should be nil after removal, got %v", profile.AvatarUrl)
	}
}

func TestUpdateUserProfileBothFields(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	client := newSessionClient(t, server.URL, 1)
	response := patchJSON(t, client, fmt.Sprintf("%s/api/v1/users/1", server.URL), `{"username":"Alice","avatarUrl":"/api/v1/media/alice.png"}`)
	defer closeBody(t, response.Body)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", response.StatusCode, http.StatusOK, readBody(t, response.Body))
	}
	var profile api.UserProfile
	if err := json.NewDecoder(response.Body).Decode(&profile); err != nil {
		t.Fatalf("decode profile: %v", err)
	}
	if profile.Username != "Alice" {
		t.Fatalf("username = %q, want Alice", profile.Username)
	}
	if profile.AvatarUrl == nil || *profile.AvatarUrl != "/api/v1/media/alice.png" {
		t.Fatalf("avatarUrl = %v", profile.AvatarUrl)
	}
}

func TestGetUserProfileWithAvatar(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	client := newSessionClient(t, server.URL, 1)
	setResponse := patchJSON(t, client, fmt.Sprintf("%s/api/v1/users/1", server.URL), `{"avatarUrl":"/api/v1/media/ava.png"}`)
	defer closeBody(t, setResponse.Body)

	getResponse, err := http.Get(fmt.Sprintf("%s/api/v1/users/1", server.URL))
	if err != nil {
		t.Fatalf("GET profile: %v", err)
	}
	defer closeBody(t, getResponse.Body)
	if getResponse.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", getResponse.StatusCode, http.StatusOK)
	}
	var profile api.UserProfile
	if err := json.NewDecoder(getResponse.Body).Decode(&profile); err != nil {
		t.Fatalf("decode profile: %v", err)
	}
	if profile.AvatarUrl == nil || *profile.AvatarUrl != "/api/v1/media/ava.png" {
		t.Fatalf("avatarUrl = %v, want /api/v1/media/ava.png", profile.AvatarUrl)
	}
}

func TestGetUserProfileWithoutAvatar(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	response, err := http.Get(fmt.Sprintf("%s/api/v1/users/1", server.URL))
	if err != nil {
		t.Fatalf("GET profile: %v", err)
	}
	defer closeBody(t, response.Body)
	var profile api.UserProfile
	if err := json.NewDecoder(response.Body).Decode(&profile); err != nil {
		t.Fatalf("decode profile: %v", err)
	}
	if profile.AvatarUrl != nil {
		t.Fatalf("avatarUrl should be nil for fresh user, got %v", profile.AvatarUrl)
	}
}

func TestItemListIsScopedToCurrentSession(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()
	firstClient := newSessionClient(t, server.URL, 1)
	secondClient := newSessionClient(t, server.URL, 2)

	firstItem := postJSON(t, firstClient, server.URL+"/api/v1/items", validItemPayload) //nolint:bodyclose // Closed below.
	closeBody(t, firstItem.Body)
	secondItem := postJSON(t, secondClient, server.URL+"/api/v1/items", validItemPayload) //nolint:bodyclose // Closed below.
	closeBody(t, secondItem.Body)

	for userID, client := range map[int64]*http.Client{1: firstClient, 2: secondClient} {
		response, err := client.Get(server.URL + "/api/v1/items") //nolint:bodyclose // Closed below.
		if err != nil {
			t.Fatalf("list items for user %d: %v", userID, err)
		}
		var list api.ItemList
		if err := json.NewDecoder(response.Body).Decode(&list); err != nil {
			closeBody(t, response.Body)
			t.Fatalf("decode items for user %d: %v", userID, err)
		}
		closeBody(t, response.Body)
		if len(list.Items) != 1 || list.Items[0].UserId != userID {
			t.Fatalf("items for user %d = %#v", userID, list.Items)
		}
	}
}

func TestCredentialedCORSUsesConfiguredOrigin(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	request, err := http.NewRequest(http.MethodOptions, server.URL+"/api/v1/session", nil)
	if err != nil {
		t.Fatalf("create preflight request: %v", err)
	}
	request.Header.Set("Origin", "http://localhost:5173")

	response, err := http.DefaultClient.Do(request) //nolint:bodyclose // Closed below.
	if err != nil {
		t.Fatalf("send preflight request: %v", err)
	}
	defer closeBody(t, response.Body)

	if got := response.Header.Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("allow origin = %q", got)
	}
	if got := response.Header.Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("allow credentials = %q", got)
	}
}

func TestItemCreatedEventIsDeliveredOnlyToOwner(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	firstClient := newSessionClient(t, server.URL, 1)
	secondClient := newSessionClient(t, server.URL, 2)

	firstResponse, firstScanner := subscribe(t, firstClient, server.URL)
	defer closeBody(t, firstResponse.Body)
	secondResponse, secondScanner := subscribe(t, secondClient, server.URL)
	defer closeBody(t, secondResponse.Body)

	waitForEvent(t, firstScanner, "stream.connected", time.Second)
	waitForEvent(t, secondScanner, "stream.connected", time.Second)

	created := postJSON(t, firstClient, server.URL+"/api/v1/items", validItemPayload)
	defer closeBody(t, created.Body)
	if created.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d, want %d; body=%s", created.StatusCode, http.StatusCreated, readBody(t, created.Body))
	}

	waitForEvent(t, firstScanner, "item.created", time.Second)

	unexpected := make(chan string, 1)
	go func() {
		for secondScanner.Scan() {
			if strings.HasPrefix(secondScanner.Text(), "event: ") {
				unexpected <- strings.TrimPrefix(secondScanner.Text(), "event: ")
				return
			}
		}
	}()

	select {
	case eventType := <-unexpected:
		t.Fatalf("second user received event %q", eventType)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestChainContractReturnsCurrentUsersChains(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()
	client := newSessionClient(t, server.URL, 1)

	response, err := client.Get(server.URL + "/api/v1/chains")
	if err != nil {
		t.Fatalf("GET chains: %v", err)
	}
	defer closeBody(t, response.Body)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", response.StatusCode, http.StatusOK, readBody(t, response.Body))
	}
	body := readBody(t, response.Body)
	if strings.Contains(body, `"imageUrls":null`) {
		t.Fatalf("chain item imageUrls must be an array; body=%s", body)
	}
	var result api.ChainList
	if err := json.Unmarshal([]byte(body), &result); err != nil {
		t.Fatalf("decode chains: %v", err)
	}
	if len(result.Chains) != 1 || result.Chains[0].Id != 42 {
		t.Fatalf("chains = %#v, want chain 42", result.Chains)
	}
	for _, participant := range result.Chains[0].Participants {
		if participant.IncomingDeliveryStatus != api.AdminDeliveryStatusAWAITINGPVZ {
			t.Fatalf("incoming delivery status = %q, want AWAITING_PVZ", participant.IncomingDeliveryStatus)
		}
		if participant.IncomingDeliveryUpdatedAt.IsZero() {
			t.Fatal("incoming delivery updated at is zero")
		}
	}
}

func TestCreateChainPassesSessionUserAndSelectedCycle(t *testing.T) {
	chainService := &testChainService{chain: testChain()}
	server := newTestServerWithChainService(t, chainService)
	defer server.Close()
	client := newSessionClient(t, server.URL, 7)

	response := postJSON(t, client, server.URL+"/api/v1/chains", validChainPayload)
	defer closeBody(t, response.Body)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", response.StatusCode, http.StatusCreated, readBody(t, response.Body))
	}
	if chainService.createUserID != 7 {
		t.Fatalf("create user ID = %d, want 7", chainService.createUserID)
	}
	want := []chains.Edge{{SourceItemID: 10, TargetItemID: 20}, {SourceItemID: 20, TargetItemID: 10}}
	if len(chainService.createInput.Edges) != len(want) {
		t.Fatalf("create edges = %#v, want %#v", chainService.createInput.Edges, want)
	}
	for index := range want {
		if chainService.createInput.Edges[index] != want[index] {
			t.Fatalf("create edge %d = %#v, want %#v", index, chainService.createInput.Edges[index], want[index])
		}
	}
}

func TestCreateChainMapsDomainErrors(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{name: "validation", err: &chains.ValidationError{Message: "invalid cycle"}, status: http.StatusBadRequest},
		{name: "forbidden", err: chains.ErrForbidden, status: http.StatusForbidden},
		{name: "not found", err: chains.ErrNotFound, status: http.StatusNotFound},
		{name: "conflict", err: chains.ErrConflict, status: http.StatusConflict},
		{name: "internal", err: errors.New("database unavailable"), status: http.StatusInternalServerError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := newTestServerWithChainService(t, &testChainService{createErr: test.err})
			defer server.Close()
			client := newSessionClient(t, server.URL, 1)

			response := postJSON(t, client, server.URL+"/api/v1/chains", validChainPayload)
			defer closeBody(t, response.Body)
			if response.StatusCode != test.status {
				t.Fatalf("status = %d, want %d; body=%s", response.StatusCode, test.status, readBody(t, response.Body))
			}
		})
	}
}

func TestOpenAPIValidationRunsBeforeHandler(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	response := postJSON(t, http.DefaultClient, server.URL+"/api/v1/items", `{"offerTitle":"only title"}`)
	defer closeBody(t, response.Body)
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", response.StatusCode, http.StatusBadRequest, readBody(t, response.Body))
	}
}

func TestMediaUploadAndPublicRead(t *testing.T) {
	storage := newTestMediaStorage()
	mediaService := media.NewService(storage, 32)
	server := newTestServerWithMedia(t, mediaService)
	defer server.Close()

	image := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52}
	unauthorized := postMultipart(t, http.DefaultClient, server.URL+"/api/v1/media", "image.png", image)
	defer closeBody(t, unauthorized.Body)
	if unauthorized.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d, want %d", unauthorized.StatusCode, http.StatusUnauthorized)
	}

	client := newSessionClient(t, server.URL, 1)
	uploaded := postMultipart(t, client, server.URL+"/api/v1/media", "image.png", image)
	defer closeBody(t, uploaded.Body)
	if uploaded.StatusCode != http.StatusCreated {
		t.Fatalf("upload status = %d, want %d; body=%s", uploaded.StatusCode, http.StatusCreated, readBody(t, uploaded.Body))
	}
	var result api.MediaUpload
	if err := json.NewDecoder(uploaded.Body).Decode(&result); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}
	if !strings.HasPrefix(result.Url, "/api/v1/media/") || result.ContentType != api.Imagepng || result.Size != int64(len(image)) {
		t.Fatalf("upload response = %+v", result)
	}

	read, err := http.Get(server.URL + result.Url)
	if err != nil {
		t.Fatalf("get media: %v", err)
	}
	defer closeBody(t, read.Body)
	if read.StatusCode != http.StatusOK || read.Header.Get("Content-Type") != "image/png" || !bytes.Equal([]byte(readBody(t, read.Body)), image) {
		t.Fatalf("read media status=%d content-type=%q", read.StatusCode, read.Header.Get("Content-Type"))
	}

	created := postJSON(t, client, server.URL+"/api/v1/items", fmt.Sprintf(`{"offerTitle":"Bike","offerDescription":"Good bike","categoryId":1,"condition":"GOOD","wishes": ["Board"],"imageUrls":[%q]}`, result.Url))
	defer closeBody(t, created.Body)
	if created.StatusCode != http.StatusCreated {
		t.Fatalf("create item status = %d, want %d; body=%s", created.StatusCode, http.StatusCreated, readBody(t, created.Body))
	}

	invalid := postJSON(t, client, server.URL+"/api/v1/items", `{"offerTitle":"Bike","offerDescription":"Good bike","categoryId":1,"condition":"GOOD","wishes": ["Board"],"imageUrls":["/api/v1/media/not-an-object.png"]}`)
	defer closeBody(t, invalid.Body)
	if invalid.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("invalid media URL status = %d, want %d; body=%s", invalid.StatusCode, http.StatusUnprocessableEntity, readBody(t, invalid.Body))
	}
}

func TestMediaUploadRejectsUnsupportedAndOversizedFiles(t *testing.T) {
	server := newTestServerWithMedia(t, media.NewService(newTestMediaStorage(), 32))
	defer server.Close()
	client := newSessionClient(t, server.URL, 1)

	unsupported := postMultipart(t, client, server.URL+"/api/v1/media", "notes.txt", []byte("plain text"))
	defer closeBody(t, unsupported.Body)
	if unsupported.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("unsupported status = %d, want %d", unsupported.StatusCode, http.StatusUnsupportedMediaType)
	}

	oversized := append([]byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}, bytes.Repeat([]byte{0}, 40)...)
	tooLarge := postMultipart(t, client, server.URL+"/api/v1/media", "large.png", oversized)
	defer closeBody(t, tooLarge.Body)
	if tooLarge.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("too large status = %d, want %d", tooLarge.StatusCode, http.StatusRequestEntityTooLarge)
	}
}

func TestVisionAnalysisDeliversTargetedSSE(t *testing.T) {
	vision := &testVisionService{result: &analyzemodel.VisualAnalysis{
		MarketplaceDescription: "Горный велосипед в хорошем состоянии",
		SuggestedCategory:      "Спорт и отдых",
		VisualQuality:          analyzemodel.Good,
		QualityScore:           0.8,
	}}
	server := newTestServerWithVision(t, vision)
	defer server.Close()
	client := newSessionClient(t, server.URL, 1)

	eventsResponse, scanner := subscribe(t, client, server.URL)
	defer closeBody(t, eventsResponse.Body)
	waitForEvent(t, scanner, "stream.connected", time.Second)

	image := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52}
	uploaded := postMultipart(t, client, server.URL+"/api/v1/media", "bike.png", image) //nolint:bodyclose // Closed below.
	var mediaResult api.MediaUpload
	if err := json.NewDecoder(uploaded.Body).Decode(&mediaResult); err != nil {
		closeBody(t, uploaded.Body)
		t.Fatalf("decode upload response: %v", err)
	}
	closeBody(t, uploaded.Body)

	accepted := postJSON(t, client, server.URL+"/api/v1/vision/analyze", fmt.Sprintf(`{"imageUrl":%q}`, mediaResult.Url))
	defer closeBody(t, accepted.Body)
	if accepted.StatusCode != http.StatusAccepted {
		t.Fatalf("analyze status = %d, want %d; body=%s", accepted.StatusCode, http.StatusAccepted, readBody(t, accepted.Body))
	}
	waitForEvent(t, scanner, "vision.analysis.completed", time.Second)
}

func TestVisionAnalysisHTTPFailures(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()
	client := newSessionClient(t, server.URL, 1)

	unauthorized := postJSON(t, http.DefaultClient, server.URL+"/api/v1/vision/analyze", `{"imageUrl":"/api/v1/media/00000000-0000-0000-0000-000000000000.jpg"}`)
	defer closeBody(t, unauthorized.Body)
	if unauthorized.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d", unauthorized.StatusCode)
	}

	unavailable := postJSON(t, client, server.URL+"/api/v1/vision/analyze", `{"imageUrl":"/api/v1/media/00000000-0000-0000-0000-000000000000.jpg"}`)
	defer closeBody(t, unavailable.Body)
	if unavailable.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("unavailable status = %d", unavailable.StatusCode)
	}

	visionServer := newTestServerWithVision(t, &testVisionService{result: &analyzemodel.VisualAnalysis{}})
	defer visionServer.Close()
	visionClient := newSessionClient(t, visionServer.URL, 1)
	notFound := postJSON(t, visionClient, visionServer.URL+"/api/v1/vision/analyze", `{"imageUrl":"/api/v1/media/00000000-0000-0000-0000-000000000000.jpg"}`)
	defer closeBody(t, notFound.Body)
	if notFound.StatusCode != http.StatusNotFound {
		t.Fatalf("not found status = %d, want %d", notFound.StatusCode, http.StatusNotFound)
	}
}

func TestVisionAnalysisRejectsWebP(t *testing.T) {
	server := newTestServerWithVision(t, &testVisionService{result: &analyzemodel.VisualAnalysis{}})
	defer server.Close()
	client := newSessionClient(t, server.URL, 1)
	webp := []byte{'R', 'I', 'F', 'F', 0x0c, 0, 0, 0, 'W', 'E', 'B', 'P', 'V', 'P', '8', ' ', 0, 0, 0, 0}
	uploaded := postMultipart(t, client, server.URL+"/api/v1/media", "image.webp", webp)
	if uploaded.StatusCode != http.StatusCreated {
		defer closeBody(t, uploaded.Body)
		t.Fatalf("upload webp status = %d; body=%s", uploaded.StatusCode, readBody(t, uploaded.Body))
	}
	var mediaResult api.MediaUpload
	if err := json.NewDecoder(uploaded.Body).Decode(&mediaResult); err != nil {
		closeBody(t, uploaded.Body)
		t.Fatalf("decode upload response: %v", err)
	}
	closeBody(t, uploaded.Body)

	response := postJSON(t, client, server.URL+"/api/v1/vision/analyze", fmt.Sprintf(`{"imageUrl":%q}`, mediaResult.Url))
	defer closeBody(t, response.Body)
	if response.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("webp status = %d, want %d; body=%s", response.StatusCode, http.StatusUnsupportedMediaType, readBody(t, response.Body))
	}
}

func TestVisionAnalysisQueueIsBounded(t *testing.T) {
	release := make(chan struct{})
	vision := &testVisionService{wait: release, result: &analyzemodel.VisualAnalysis{}}
	server := newTestServerWithVision(t, vision)
	defer server.Close()
	client := newSessionClient(t, server.URL, 1)
	image := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52}
	uploaded := postMultipart(t, client, server.URL+"/api/v1/media", "image.png", image) //nolint:bodyclose // Closed below.
	var mediaResult api.MediaUpload
	if err := json.NewDecoder(uploaded.Body).Decode(&mediaResult); err != nil {
		closeBody(t, uploaded.Body)
		t.Fatalf("decode upload response: %v", err)
	}
	closeBody(t, uploaded.Body)
	payload := fmt.Sprintf(`{"imageUrl":%q}`, mediaResult.Url)

	for i := 0; i < 4; i++ {
		response := postJSON(t, client, server.URL+"/api/v1/vision/analyze", payload) //nolint:bodyclose // Closed on every branch below.
		if response.StatusCode != http.StatusAccepted {
			closeBody(t, response.Body)
			close(release)
			t.Fatalf("request %d status = %d, want %d", i, response.StatusCode, http.StatusAccepted)
		}
		closeBody(t, response.Body)
	}
	overflow := postJSON(t, client, server.URL+"/api/v1/vision/analyze", payload)
	defer closeBody(t, overflow.Body)
	close(release)
	if overflow.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("overflow status = %d, want %d", overflow.StatusCode, http.StatusTooManyRequests)
	}
}

func newTestServer(t *testing.T) *httptest.Server {
	return newTestServerWithChainService(t, &testChainService{chain: testChain()})
}

func newTestServerWithChainService(t *testing.T, chainService chains.Service) *httptest.Server {
	return newTestServerWithServices(t, chainService, nil)
}

func newTestServerWithServices(t *testing.T, chainService chains.Service, itemService items.Service) *httptest.Server {
	return newTestServerWithAllServices(t, chainService, itemService, media.NewService(newTestMediaStorage(), 10<<20))
}

func newTestServerWithMedia(t *testing.T, mediaService *media.Service) *httptest.Server {
	return newTestServerWithAllServices(t, &testChainService{chain: testChain()}, nil, mediaService)
}

func newTestServerWithAllServices(t *testing.T, chainService chains.Service, itemService items.Service, mediaService *media.Service) *httptest.Server {
	return newTestServerWithAllServicesAndVision(t, chainService, itemService, mediaService, nil)
}

func newTestServerWithVision(t *testing.T, vision httpapi.VisionService) *httptest.Server {
	return newTestServerWithAllServicesAndVision(t, &testChainService{chain: testChain()}, nil, media.NewService(newTestMediaStorage(), 10<<20), vision)
}

func newTestServerWithAllServicesAndVision(t *testing.T, chainService chains.Service, itemService items.Service, mediaService *media.Service, vision httpapi.VisionService) *httptest.Server {
	t.Helper()
	logger := zap.NewNop()
	eventHub := events.NewHub()
	sessions := session.NewManager(time.Hour, false)
	userService := users.NewMemoryService(
		testUser(1),
		testUser(2),
		testUser(7),
	)
	if itemService == nil {
		itemService = items.NewMemoryService(func(userID int64, eventType, entityID string, data map[string]any) {
			eventHub.PublishToUser(userID, eventType, entityID, data)
		})
	}
	handler := httpapi.NewHandler(testDatabase{}, testFinder{}, logger, itemService, mediaService, chainService, &testCategoriesService{}, eventHub, sessions, userService, &testAdminService{}, newTestChatService(), &testBlocklistService{}, &testModerationService{}, nil, &testReputationService{}, &testMetricsService{}, vision)
	router, err := New(logger, handler, sessions, "http://localhost:5173", 10<<20)
	if err != nil {
		t.Fatalf("create HTTP handler: %v", err)
	}
	return httptest.NewServer(router)
}

type testVisionService struct {
	result *analyzemodel.VisualAnalysis
	err    error
	wait   <-chan struct{}
}

func (s *testVisionService) DescribeImage(ctx context.Context, _ []byte) (*analyzemodel.VisualAnalysis, error) {
	if s.wait != nil {
		select {
		case <-s.wait:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return s.result, s.err
}

func postMultipart(t *testing.T, client *http.Client, url, filename string, data []byte) *http.Response {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create multipart file: %v", err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatalf("write multipart file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	request, err := http.NewRequest(http.MethodPost, url, &body)
	if err != nil {
		t.Fatalf("create multipart request: %v", err)
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("post multipart: %v", err)
	}
	return response
}

func newSessionClient(t *testing.T, baseURL string, userID int64) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("create cookie jar: %v", err)
	}
	client := &http.Client{Jar: jar}
	response := postJSON(t, client, baseURL+"/api/v1/session", fmt.Sprintf(`{"phone":%q}`, phoneForUserID(userID)))
	defer closeBody(t, response.Body)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("set session status = %d, want %d; body=%s", response.StatusCode, http.StatusOK, readBody(t, response.Body))
	}
	return client
}

func testUser(userID int64) users.User {
	role := users.RoleUser
	if userID == 7 {
		role = users.RoleAdmin
	}
	return users.User{
		ID:        userID,
		Username:  fmt.Sprintf("user-%d", userID),
		Phone:     phoneForUserID(userID),
		Role:      role,
		CreatedAt: time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC),
	}
}

func phoneForUserID(userID int64) string {
	return fmt.Sprintf("+7%010d", userID)
}

func subscribe(t *testing.T, client *http.Client, baseURL string) (*http.Response, *bufio.Scanner) {
	t.Helper()
	response, err := client.Get(baseURL + "/api/v1/events")
	if err != nil {
		t.Fatalf("subscribe events: %v", err)
	}
	if response.StatusCode != http.StatusOK {
		defer closeBody(t, response.Body)
		t.Fatalf("subscribe status = %d, want %d; body=%s", response.StatusCode, http.StatusOK, readBody(t, response.Body))
	}
	return response, bufio.NewScanner(response.Body)
}

func waitForEvent(t *testing.T, scanner *bufio.Scanner, eventType string, timeout time.Duration) {
	t.Helper()
	found := make(chan bool, 1)
	go func() {
		for scanner.Scan() {
			if scanner.Text() == "event: "+eventType {
				found <- true
				return
			}
		}
		found <- false
	}()

	select {
	case ok := <-found:
		if !ok {
			t.Fatalf("event %q not found: %v", eventType, scanner.Err())
		}
	case <-time.After(timeout):
		t.Fatalf("timed out waiting for event %q", eventType)
	}
}

func postJSON(t *testing.T, client *http.Client, url, payload string) *http.Response {
	t.Helper()
	return sendJSON(t, client, http.MethodPost, url, payload)
}

func patchJSON(t *testing.T, client *http.Client, url, payload string) *http.Response {
	t.Helper()
	return sendJSON(t, client, http.MethodPatch, url, payload)
}

func sendJSON(t *testing.T, client *http.Client, method, url, payload string) *http.Response {
	t.Helper()
	request, err := http.NewRequest(method, url, bytes.NewBufferString(payload))
	if err != nil {
		t.Fatalf("create %s %s: %v", method, url, err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	return response
}

func readBody(t *testing.T, reader io.Reader) string {
	t.Helper()
	value, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(value)
}

func closeBody(t *testing.T, body io.Closer) {
	t.Helper()
	if err := body.Close(); err != nil {
		t.Errorf("close response body: %v", err)
	}
}

type testDatabase struct{}

func (testDatabase) PingContext(context.Context) error { return nil }

type testReadiness struct {
	err error
}

func (readiness testReadiness) Ready(context.Context) error { return readiness.err }

type testMediaStorage struct {
	objects map[string]media.Object
	data    map[string][]byte
}

func newTestMediaStorage() *testMediaStorage {
	return &testMediaStorage{objects: make(map[string]media.Object), data: make(map[string][]byte)}
}

func (s *testMediaStorage) Put(_ context.Context, key string, body io.Reader, size int64, contentType string) error {
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	s.objects[key] = media.Object{Key: key, ContentType: contentType, Size: size}
	s.data[key] = data
	return nil
}

func (s *testMediaStorage) Open(_ context.Context, key string) (media.Object, error) {
	object, ok := s.objects[key]
	if !ok {
		return media.Object{}, media.ErrNotFound
	}
	object.Body = io.NopCloser(bytes.NewReader(s.data[key]))
	return object, nil
}

type matchingReadyItems struct {
	items.Service
}

func (service matchingReadyItems) Get(ctx context.Context, itemID int64) (items.Item, error) {
	item, err := service.Service.Get(ctx, itemID)
	if err == nil {
		item.Status = "MATCHING"
	}
	return item, err
}

type testFinder struct{}

func (testFinder) Execute(_ context.Context, itemID int64) ([][]model.Edge, error) {
	return [][]model.Edge{{
		{SourceID: itemID, TargetID: itemID + 1, Score: 0.8},
		{SourceID: itemID + 1, TargetID: itemID, Score: 0.9},
	}}, nil
}

type testCategoriesService struct {
	validateErr error
}

func (s *testCategoriesService) List(_ context.Context) ([]categories.Category, error) {
	return []categories.Category{
		{ID: 1, Name: "Электроника", IsSystem: true},
		{ID: 2, Name: "Бытовая техника", IsSystem: true},
		{ID: 46, Name: "Прочее", IsSystem: false},
	}, nil
}

func (s *testCategoriesService) ValidateCategory(_ context.Context, categoryID int32) error {
	if s.validateErr != nil {
		return s.validateErr
	}
	if categoryID == 47 {
		return categories.ErrUndefinedCategory
	}
	for _, c := range []int32{1, 2, 46} {
		if c == categoryID {
			return nil
		}
	}
	return categories.ErrCategoryNotFound
}

type testChainService struct {
	chain        chains.Chain
	createUserID int64
	createInput  chains.CreateInput
	createErr    error
}

type testAdminService struct{}

type testBlocklistService struct{}

type testModerationService struct{}

func (*testBlocklistService) Block(_ context.Context, blockerID, blockedID int64) (blocklistmodel.Block, error) {
	if blockerID == blockedID {
		return blocklistmodel.Block{}, blocklistmodel.ErrSelfBlock
	}
	return blocklistmodel.Block{
		ID: 1,
		BlockedUser: blocklistmodel.BlockedUser{
			ID:       blockedID,
			Username: fmt.Sprintf("user-%d", blockedID),
		},
		BlockedAt: time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC),
	}, nil
}

func (*testBlocklistService) Unblock(_ context.Context, _, _ int64) error {
	return nil
}

func (*testBlocklistService) List(_ context.Context, _ int64, _ int64, _ int) ([]blocklistmodel.Block, *int64, error) {
	return []blocklistmodel.Block{}, nil, nil
}

func (*testModerationService) CreateReport(_ context.Context, reporterID, messageID int64, reason, _ string) (moderationmodel.Report, bool, error) {
	return moderationmodel.Report{
		ID:        1,
		Reporter:  moderationmodel.User{ID: reporterID, Username: fmt.Sprintf("user-%d", reporterID)},
		MessageID: messageID,
		Reason:    reason,
		Status:    moderationmodel.ReportOpen,
		CreatedAt: time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC),
	}, true, nil
}

func (*testModerationService) CreateUserReport(_ context.Context, reporterID, targetID int64, chainID *int64, reason, comment string) (moderationmodel.UserReport, bool, error) {
	var normalizedComment *string
	if comment != "" {
		normalizedComment = &comment
	}
	return moderationmodel.UserReport{
		ID:         2,
		ReporterID: reporterID,
		TargetID:   targetID,
		ChainID:    chainID,
		Reason:     reason,
		Comment:    normalizedComment,
		CreatedAt:  time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC),
	}, true, nil
}

func (*testModerationService) ListReports(_ context.Context, _ int64, _ moderationmodel.ReportFilter, _ int64, _ int) ([]moderationmodel.Report, *int64, error) {
	return []moderationmodel.Report{}, nil, nil
}

func (*testModerationService) GetReport(_ context.Context, _, _ int64) (moderationmodel.ReportDetail, error) {
	return moderationmodel.ReportDetail{}, nil
}

func (*testModerationService) Assign(_ context.Context, _, reportID int64) (moderationmodel.Report, error) {
	return moderationmodel.Report{ID: reportID, Status: moderationmodel.ReportOpen}, nil
}

func (*testModerationService) Decide(_ context.Context, _, reportID int64, decision, comment string) (moderationmodel.Report, error) {
	commentValue := comment
	return moderationmodel.Report{ID: reportID, Status: decision, DecisionComment: &commentValue}, nil
}

func (*testModerationService) ListAudit(_ context.Context, _ int64, _ moderationmodel.AuditFilter, _ *int64, _ int) ([]moderationmodel.AuditEntry, *int64, error) {
	return []moderationmodel.AuditEntry{}, nil, nil
}

type testReputationService struct{}

type testMetricsService struct{}

func (*testMetricsService) Funnel(_ context.Context, actorID int64) (metricsmodel.Funnel, error) {
	if actorID != 7 {
		return metricsmodel.Funnel{}, metricsmodel.ErrForbidden
	}
	rate := 0.5
	return metricsmodel.Funnel{
		GeneratedAt:                    time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC),
		EligibleItems:                  10,
		ItemsWithChain:                 5,
		ItemsWithChainRate:             &rate,
		AverageTimeToFirstChainSeconds: &rate,
		DecidedChains:                  4,
		AcceptedChains:                 2,
		AcceptanceRate:                 &rate,
		CompletedChains:                1,
		DeliveryCompletionRate:         &rate,
		RejectionReasons:               []metricsmodel.RejectionReason{{Reason: "declined", Count: 2}},
	}, nil
}

func (*testReputationService) Stats(_ context.Context, _ int64) (reputationmodel.Stats, error) {
	rating := 4.5
	return reputationmodel.Stats{CompletedExchanges: 2, Rating: &rating, ReviewsCount: 3}, nil
}

func (*testReputationService) CreateReview(_ context.Context, authorID, chainID int64, input reputationmodel.CreateInput) (reputationmodel.Review, error) {
	return reputationmodel.Review{
		ID:           1,
		ChainID:      chainID,
		Author:       reputationmodel.Author{ID: authorID, Username: fmt.Sprintf("user-%d", authorID)},
		TargetUserID: input.TargetUserID,
		Rating:       input.Rating,
		Text:         input.Text,
		CreatedAt:    time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC),
	}, nil
}

func (*testReputationService) ListReviews(_ context.Context, _ int64, _ int64, _ int) (reputationmodel.ListResult, error) {
	return reputationmodel.ListResult{Reviews: []reputationmodel.Review{}}, nil
}

type testChatService struct {
	mu       sync.Mutex
	nextID   int64
	messages map[string]chatmodel.Message
	reads    map[string]int64
}

func newTestChatService() *testChatService {
	return &testChatService{messages: make(map[string]chatmodel.Message), reads: make(map[string]int64)}
}

func (s *testChatService) Send(_ context.Context, itemID, actorID, counterpartID int64, clientMessageID, text string) (chatmodel.Message, bool, error) {
	if err := testChatAccess(itemID, actorID, counterpartID); err != nil {
		return chatmodel.Message{}, false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := fmt.Sprintf("%d:%d:%d:%s", itemID, actorID, counterpartID, clientMessageID)
	if existing, ok := s.messages[key]; ok {
		if existing.Text != text {
			return chatmodel.Message{}, false, chatmodel.ErrIdempotencyConflict
		}
		return existing, false, nil
	}
	s.nextID++
	message := chatmodel.Message{
		ID:              s.nextID,
		ItemID:          itemID,
		OriginChainID:   42,
		Sender:          chatmodel.Sender{ID: actorID, Username: fmt.Sprintf("user-%d", actorID)},
		Recipient:       chatmodel.Sender{ID: counterpartID, Username: fmt.Sprintf("user-%d", counterpartID)},
		ClientMessageID: clientMessageID,
		Text:            text,
		CreatedAt:       time.Date(2026, 8, 10, 12, 30, 0, 0, time.UTC),
	}
	s.messages[key] = message
	return message, true, nil
}

func (s *testChatService) List(_ context.Context, itemID, actorID, counterpartID, afterID int64, limit int, _ time.Duration) ([]chatmodel.Message, error) {
	if err := testChatAccess(itemID, actorID, counterpartID); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	messages := make([]chatmodel.Message, 0, limit)
	for _, message := range s.messages {
		isThreadMessage := (message.Sender.ID == actorID && message.Recipient.ID == counterpartID) ||
			(message.Sender.ID == counterpartID && message.Recipient.ID == actorID)
		if message.ItemID == itemID && isThreadMessage && message.ID > afterID {
			messages = append(messages, message)
		}
	}
	sort.Slice(messages, func(i, j int) bool { return messages[i].ID < messages[j].ID })
	if len(messages) > limit {
		messages = messages[:limit]
	}
	return messages, nil
}

func (s *testChatService) ListThreads(_ context.Context, actorID int64) ([]chatmodel.Thread, error) {
	if actorID != 1 && actorID != 2 {
		return nil, chatmodel.ErrForbidden
	}
	counterpartID := int64(1)
	if actorID == 1 {
		counterpartID = 2
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	thread := chatmodel.Thread{
		Item:        chatmodel.ItemSummary{ID: 202, Title: "Получаемая вещь", ImageURL: "/api/v1/media/receive.jpg"},
		Counterpart: chatmodel.Sender{ID: counterpartID, Username: fmt.Sprintf("user-%d", counterpartID)},
	}
	watermark := s.reads[fmt.Sprintf("%d:%d:%d", 202, actorID, counterpartID)]
	for _, message := range s.messages {
		isThreadMessage := message.ItemID == 202 && ((message.Sender.ID == actorID && message.Recipient.ID == counterpartID) ||
			(message.Sender.ID == counterpartID && message.Recipient.ID == actorID))
		if !isThreadMessage {
			continue
		}
		if thread.LastMessage == nil || message.ID > thread.LastMessage.ID {
			messageCopy := message
			thread.LastMessage = &messageCopy
		}
		if message.Sender.ID == counterpartID && message.ID > watermark {
			thread.UnreadCount++
		}
	}
	return []chatmodel.Thread{thread}, nil
}

func (s *testChatService) MarkRead(_ context.Context, itemID, actorID, counterpartID, lastReadMessageID int64) (chatmodel.ReadState, error) {
	if err := testChatAccess(itemID, actorID, counterpartID); err != nil {
		return chatmodel.ReadState{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	belongs := false
	for _, message := range s.messages {
		if message.ID == lastReadMessageID && message.ItemID == itemID &&
			((message.Sender.ID == actorID && message.Recipient.ID == counterpartID) ||
				(message.Sender.ID == counterpartID && message.Recipient.ID == actorID)) {
			belongs = true
			break
		}
	}
	if !belongs {
		return chatmodel.ReadState{}, chatmodel.ErrMessageNotFound
	}
	key := fmt.Sprintf("%d:%d:%d", itemID, actorID, counterpartID)
	if lastReadMessageID > s.reads[key] {
		s.reads[key] = lastReadMessageID
	}
	unread := int64(0)
	for _, message := range s.messages {
		if message.ItemID == itemID && message.Sender.ID == counterpartID && message.Recipient.ID == actorID && message.ID > s.reads[key] {
			unread++
		}
	}
	return chatmodel.ReadState{ItemID: itemID, CounterpartID: counterpartID, LastReadMessageID: s.reads[key], UnreadCount: unread}, nil
}

func testChatAccess(itemID, actorID, counterpartID int64) error {
	switch {
	case itemID == 404:
		return chatmodel.ErrItemNotFound
	case actorID != 1 && actorID != 2:
		return chatmodel.ErrForbidden
	case counterpartID != 1 && counterpartID != 2, actorID == counterpartID:
		return chatmodel.ErrThreadNotFound
	default:
		return nil
	}
}

func (*testAdminService) ListDeliveries(_ context.Context, actorID int64, _ string, _ int64, _ int) ([]adminmodel.Delivery, *int64, error) {
	if actorID != 7 {
		return nil, nil, adminmodel.ErrForbidden
	}
	return []adminmodel.Delivery{testAdminDelivery()}, nil, nil
}

func (*testAdminService) TransitionDelivery(_ context.Context, actorID, deliveryID int64, targetStatus string) (adminmodel.Delivery, error) {
	if actorID != 7 {
		return adminmodel.Delivery{}, adminmodel.ErrForbidden
	}
	switch deliveryID {
	case 404:
		return adminmodel.Delivery{}, adminmodel.ErrDeliveryNotFound
	case 42:
		return adminmodel.Delivery{}, adminmodel.ErrTransitionConflict
	}
	delivery := testAdminDelivery()
	delivery.Status = targetStatus
	return delivery, nil
}

func (*testAdminService) ConfirmReceipt(_ context.Context, actorID, chainID int64) (adminmodel.Receipt, error) {
	switch {
	case chainID == 404:
		return adminmodel.Receipt{}, adminmodel.ErrChainNotFound
	case actorID != 2:
		return adminmodel.Receipt{}, adminmodel.ErrReceiptForbidden
	case chainID == 43:
		return adminmodel.Receipt{}, adminmodel.ErrTransitionConflict
	}
	delivery := testAdminDelivery()
	delivery.Status = adminmodel.DeliveryReceived
	return adminmodel.Receipt{Delivery: delivery, ChainStatus: adminmodel.ChainCompleted}, nil
}

func testAdminDelivery() adminmodel.Delivery {
	return adminmodel.Delivery{
		ID:                41,
		ChainID:           12,
		ItemID:            55,
		ItemTitle:         "Samsung Galaxy M54 5G",
		SenderID:          1,
		SenderUsername:    "Алиса",
		RecipientID:       2,
		RecipientUsername: "Вера",
		Status:            adminmodel.DeliveryAwaitingPVZ,
		UpdatedAt:         time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC),
	}
}

func (s *testChainService) Create(_ context.Context, userID int64, input chains.CreateInput) (chains.Chain, error) {
	s.createUserID = userID
	s.createInput = input
	return s.chain, s.createErr
}

func (s *testChainService) List(context.Context, int64, string, int64, int) ([]chains.Chain, *int64, error) {
	return []chains.Chain{s.chain}, nil, nil
}

func (s *testChainService) Get(context.Context, int64, int64) (chains.Chain, error) {
	return s.chain, nil
}

func (s *testChainService) Decide(context.Context, int64, int64, string) (chains.Chain, error) {
	return s.chain, nil
}

func (*testChainService) ExpirePending(context.Context, int) (int, error) { return 0, nil }

func testChain() chains.Chain {
	now := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	first := items.Item{ID: 10, UserID: 1, OfferTitle: "Books", Status: "MATCHING", CreatedAt: now, UpdatedAt: now}
	second := items.Item{ID: 20, UserID: 2, OfferTitle: "Game", Status: "MATCHING", CreatedAt: now, UpdatedAt: now}
	return chains.Chain{
		ID:        42,
		Status:    chains.StatusPending,
		CreatedAt: now,
		ExpiresAt: now.Add(24 * time.Hour),
		Participants: []chains.Participant{
			{User: chains.User{ID: 1, Username: "first"}, GiveItem: first, ReceiveItem: second, Status: chains.ParticipantApproved, IncomingDeliveryStatus: adminmodel.DeliveryAwaitingPVZ, IncomingDeliveryUpdatedAt: now},
			{User: chains.User{ID: 2, Username: "second"}, GiveItem: second, ReceiveItem: first, Status: chains.ParticipantWaiting, IncomingDeliveryStatus: adminmodel.DeliveryAwaitingPVZ, IncomingDeliveryUpdatedAt: now},
		},
	}
}

const validItemPayload = `{
  "offerTitle": "Books",
  "offerDescription": "A set of science fiction books",
  "categoryId": 1,
  "condition": "GOOD",
  "wishes": ["A strategy board game"]
}`

const validChainPayload = `{
  "edges": [
    {"sourceItemId": 10, "targetItemId": 20},
    {"sourceItemId": 20, "targetItemId": 10}
  ]
}`

const validChatPayload = `{
  "clientMessageId": "web-message-1",
  "text": "Привет участникам!"
}`

type testNotificationService struct {
	notifications []notificationmodel.Notification
	nextID        int64
	mu            sync.Mutex
}

func newTestNotificationService() *testNotificationService {
	return &testNotificationService{
		notifications: make([]notificationmodel.Notification, 0),
		nextID:        1,
	}
}

func mustCreateNotification(
	t *testing.T,
	svc *testNotificationService,
	userID int64,
	kind, title, text string,
	chainID, itemID *int64,
) notificationmodel.Notification {
	t.Helper()
	notification, err := svc.Create(context.Background(), userID, kind, title, text, chainID, itemID)
	if err != nil {
		t.Fatalf("create test notification: %v", err)
	}
	return notification
}

func (s *testNotificationService) Create(_ context.Context, userID int64, kind, title, text string, chainID, itemID *int64) (notificationmodel.Notification, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := notificationmodel.Notification{
		ID:      s.nextID,
		UserID:  userID,
		Kind:    kind,
		Title:   title,
		Text:    text,
		ChainID: chainID,
		ItemID:  itemID,
		Read:    false,
	}
	s.nextID++
	s.notifications = append([]notificationmodel.Notification{n}, s.notifications...)
	return n, nil
}

func (s *testNotificationService) List(_ context.Context, userID int64, cursor int64, limit int) (notificationmodel.ListResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var filtered []notificationmodel.Notification
	for _, n := range s.notifications {
		if n.UserID == userID && (cursor == 0 || n.ID < cursor) {
			filtered = append(filtered, n)
		}
	}

	var unreadCount int64
	for _, n := range s.notifications {
		if n.UserID == userID && !n.Read {
			unreadCount++
		}
	}

	if len(filtered) > limit {
		last := filtered[limit-1]
		return notificationmodel.ListResult{
			Notifications: filtered[:limit],
			NextCursor:    &last.ID,
			TotalUnread:   unreadCount,
		}, nil
	}

	return notificationmodel.ListResult{
		Notifications: filtered,
		NextCursor:    nil,
		TotalUnread:   unreadCount,
	}, nil
}

func (s *testNotificationService) MarkRead(_ context.Context, userID int64, ids []int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(ids) == 0 {
		for i := range s.notifications {
			if s.notifications[i].UserID == userID {
				s.notifications[i].Read = true
			}
		}
		return nil
	}

	for i := range s.notifications {
		if s.notifications[i].UserID != userID {
			continue
		}
		for _, id := range ids {
			if s.notifications[i].ID == id {
				s.notifications[i].Read = true
				break
			}
		}
	}
	return nil
}

var _ notificationservice.Service = (*testNotificationService)(nil)

func newTestServerWithNotifications(t *testing.T, notificationService notificationservice.Service) *httptest.Server {
	t.Helper()
	logger := zap.NewNop()
	eventHub := events.NewHub()
	sessions := session.NewManager(time.Hour, false)
	userService := users.NewMemoryService(
		testUser(1),
		testUser(2),
		testUser(3),
		testUser(7),
	)
	itemService := items.NewMemoryService(func(userID int64, eventType, entityID string, data map[string]any) {
		eventHub.PublishToUser(userID, eventType, entityID, data)
	})
	mediaService := media.NewService(newTestMediaStorage(), 10<<20)
	chainService := &testChainService{chain: testChain()}

	handler := httpapi.NewHandler(
		testDatabase{},
		testFinder{},
		logger,
		itemService,
		mediaService,
		chainService,
		&testCategoriesService{},
		eventHub,
		sessions,
		userService,
		&testAdminService{},
		newTestChatService(),
		&testBlocklistService{},
		&testModerationService{},
		notificationService,
		&testReputationService{},
		&testMetricsService{},
		nil,
	)
	router, err := New(logger, handler, sessions, "http://localhost:5173", 10<<20)
	if err != nil {
		t.Fatalf("create HTTP handler: %v", err)
	}
	return httptest.NewServer(router)
}

func TestListNotificationsRequiresSession(t *testing.T) {
	svc := newTestNotificationService()
	server := newTestServerWithNotifications(t, svc)
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/v1/notifications")
	if err != nil {
		t.Fatalf("GET notifications: %v", err)
	}
	defer closeBody(t, resp.Body)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestMarkNotificationsReadRequiresSession(t *testing.T) {
	svc := newTestNotificationService()
	server := newTestServerWithNotifications(t, svc)
	defer server.Close()

	resp, err := http.Post(server.URL+"/api/v1/notifications/read", "application/json", strings.NewReader(`{"ids": [1]}`))
	if err != nil {
		t.Fatalf("POST notifications/read: %v", err)
	}
	defer closeBody(t, resp.Body)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestBlocklistAndModerationRoutesRequireSession(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	paths := []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodGet, path: "/api/v1/blocks"},
		{method: http.MethodPost, path: "/api/v1/blocks", body: `{"blockedUserId":2}`},
		{method: http.MethodDelete, path: "/api/v1/blocks/2"},
		{method: http.MethodPost, path: "/api/v1/reports", body: `{"messageId":1,"reason":"spam"}`},
		{method: http.MethodPost, path: "/api/v1/users/2/reports", body: `{"reason":"rude"}`},
		{method: http.MethodGet, path: "/api/v1/admin/reports"},
		{method: http.MethodGet, path: "/api/v1/admin/reports/1"},
		{method: http.MethodPost, path: "/api/v1/admin/reports/1/assign"},
		{method: http.MethodPost, path: "/api/v1/admin/reports/1/decision", body: `{"decision":"resolved","comment":"ok"}`},
		{method: http.MethodGet, path: "/api/v1/admin/audit"},
	}
	for _, test := range paths {
		func() {
			var request *http.Request
			var err error
			if test.body == "" {
				request, err = http.NewRequest(test.method, server.URL+test.path, nil)
			} else {
				request, err = http.NewRequest(test.method, server.URL+test.path, strings.NewReader(test.body))
				request.Header.Set("Content-Type", "application/json")
			}
			if err != nil {
				t.Fatalf("new request: %v", err)
			}
			response, err := http.DefaultClient.Do(request)
			if err != nil {
				t.Fatalf("%s %s: %v", test.method, test.path, err)
			}
			defer closeBody(t, response.Body)
			if response.StatusCode != http.StatusUnauthorized {
				t.Fatalf("%s %s status = %d, want 401; body=%s", test.method, test.path, response.StatusCode, readBody(t, response.Body))
			}
		}()
	}
}

func TestBlockAndReportRoutesWithSession(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	client := newSessionClient(t, server.URL, 1)

	blockResponse := postJSON(t, client, server.URL+"/api/v1/blocks", `{"blockedUserId":2}`)
	defer closeBody(t, blockResponse.Body)
	if blockResponse.StatusCode != http.StatusOK {
		t.Fatalf("POST blocks status = %d, want 200; body=%s", blockResponse.StatusCode, readBody(t, blockResponse.Body))
	}
	var block api.Block
	if err := json.NewDecoder(blockResponse.Body).Decode(&block); err != nil {
		t.Fatalf("decode block: %v", err)
	}
	if block.BlockedUser.Id != 2 {
		t.Fatalf("blocked user = %d, want 2", block.BlockedUser.Id)
	}

	listResponse, err := client.Get(server.URL + "/api/v1/blocks")
	if err != nil {
		t.Fatalf("GET blocks: %v", err)
	}
	defer closeBody(t, listResponse.Body)
	if listResponse.StatusCode != http.StatusOK {
		t.Fatalf("GET blocks status = %d, want 200", listResponse.StatusCode)
	}

	selfResponse := postJSON(t, client, server.URL+"/api/v1/blocks", `{"blockedUserId":1}`)
	defer closeBody(t, selfResponse.Body)
	if selfResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("self-block status = %d, want 400; body=%s", selfResponse.StatusCode, readBody(t, selfResponse.Body))
	}

	reportResponse := postJSON(t, client, server.URL+"/api/v1/reports", `{"messageId":7,"reason":"spam"}`)
	defer closeBody(t, reportResponse.Body)
	if reportResponse.StatusCode != http.StatusCreated {
		t.Fatalf("POST reports status = %d, want 201; body=%s", reportResponse.StatusCode, readBody(t, reportResponse.Body))
	}

	userReportResponse := postJSON(t, client, server.URL+"/api/v1/users/2/reports", `{"reason":"rude","comment":"оскорбления"}`)
	defer closeBody(t, userReportResponse.Body)
	if userReportResponse.StatusCode != http.StatusCreated {
		t.Fatalf("POST user report status = %d, want 201; body=%s", userReportResponse.StatusCode, readBody(t, userReportResponse.Body))
	}
	var userReport api.UserReport
	if err := json.NewDecoder(userReportResponse.Body).Decode(&userReport); err != nil {
		t.Fatalf("decode user report: %v", err)
	}
	if userReport.ReporterUserId != 1 || userReport.TargetUserId != 2 || userReport.Reason != api.UserReportReasonRude {
		t.Fatalf("user report = %+v, want reporter=1 target=2 reason=rude", userReport)
	}
}

func TestListNotifications_Empty(t *testing.T) {
	svc := newTestNotificationService()
	server := newTestServerWithNotifications(t, svc)
	defer server.Close()

	client := newSessionClient(t, server.URL, 1)
	resp, err := client.Get(server.URL + "/api/v1/notifications")
	if err != nil {
		t.Fatalf("GET notifications: %v", err)
	}
	defer closeBody(t, resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var list api.NotificationList
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(list.Notifications) != 0 {
		t.Errorf("expected 0 items, got %d", len(list.Notifications))
	}
	if list.TotalUnread != 0 {
		t.Errorf("expected 0 unread, got %d", list.TotalUnread)
	}
}

func TestListNotifications_WithData(t *testing.T) {
	svc := newTestNotificationService()
	chainID := int64(42)
	mustCreateNotification(t, svc, 1, "CHAIN", "Title 1", "Text 1", &chainID, nil)
	mustCreateNotification(t, svc, 1, "MESSAGE", "Title 2", "Text 2", &chainID, nil)
	mustCreateNotification(t, svc, 2, "CHAIN", "Other user", "Other", &chainID, nil)

	server := newTestServerWithNotifications(t, svc)
	defer server.Close()

	client := newSessionClient(t, server.URL, 1)
	resp, err := client.Get(server.URL + "/api/v1/notifications")
	if err != nil {
		t.Fatalf("GET notifications: %v", err)
	}
	defer closeBody(t, resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var list api.NotificationList
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(list.Notifications) != 2 {
		t.Fatalf("expected 2 items, got %d", len(list.Notifications))
	}
	if list.TotalUnread != 2 {
		t.Errorf("expected 2 unread, got %d", list.TotalUnread)
	}
}

func TestMarkNotificationsRead_MarkAll(t *testing.T) {
	svc := newTestNotificationService()
	cID := int64(1)
	mustCreateNotification(t, svc, 1, "CHAIN", "T1", "Text", &cID, nil)
	mustCreateNotification(t, svc, 1, "CHAIN", "T2", "Text", &cID, nil)

	server := newTestServerWithNotifications(t, svc)
	defer server.Close()

	client := newSessionClient(t, server.URL, 1)
	resp := postJSON(t, client, server.URL+"/api/v1/notifications/read", `{}`)
	defer closeBody(t, resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var list api.NotificationList
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if list.TotalUnread != 0 {
		t.Errorf("expected 0 unread after mark all, got %d", list.TotalUnread)
	}
}

func TestMarkNotificationsRead_MarkSelected(t *testing.T) {
	svc := newTestNotificationService()
	cID := int64(1)
	n1 := mustCreateNotification(t, svc, 1, "CHAIN", "T1", "Text", &cID, nil)
	mustCreateNotification(t, svc, 1, "CHAIN", "T2", "Text", &cID, nil)

	server := newTestServerWithNotifications(t, svc)
	defer server.Close()

	client := newSessionClient(t, server.URL, 1)
	body := fmt.Sprintf(`{"ids": [%d]}`, n1.ID)
	resp := postJSON(t, client, server.URL+"/api/v1/notifications/read", body)
	defer closeBody(t, resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var list api.NotificationList
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if list.TotalUnread != 1 {
		t.Errorf("expected 1 unread after marking one, got %d", list.TotalUnread)
	}
}

func TestListNotifications_CursorPagination(t *testing.T) {
	svc := newTestNotificationService()
	cID := int64(1)
	for i := 0; i < 5; i++ {
		mustCreateNotification(t, svc, 1, "CHAIN",
			fmt.Sprintf("T%d", i), "Text", &cID, nil)
	}

	server := newTestServerWithNotifications(t, svc)
	defer server.Close()

	client := newSessionClient(t, server.URL, 1)

	resp, err := client.Get(server.URL + "/api/v1/notifications?limit=3")
	if err != nil {
		t.Fatalf("GET notifications page 1: %v", err)
	}
	defer closeBody(t, resp.Body)

	var page1 api.NotificationList
	if err := json.NewDecoder(resp.Body).Decode(&page1); err != nil {
		t.Fatalf("decode page1: %v", err)
	}
	if len(page1.Notifications) != 3 {
		t.Fatalf("expected 3 items on page 1, got %d", len(page1.Notifications))
	}
	if page1.NextCursor == nil {
		t.Fatal("expected nextCursor on page 1")
	}

	resp2, err := client.Get(server.URL + "/api/v1/notifications?limit=3&cursor=" + *page1.NextCursor)
	if err != nil {
		t.Fatalf("GET notifications page 2: %v", err)
	}
	defer closeBody(t, resp2.Body)

	var page2 api.NotificationList
	if err := json.NewDecoder(resp2.Body).Decode(&page2); err != nil {
		t.Fatalf("decode page2: %v", err)
	}
	if len(page2.Notifications) != 2 {
		t.Fatalf("expected 2 items on page 2, got %d", len(page2.Notifications))
	}
	if page2.NextCursor != nil {
		t.Errorf("expected nil nextCursor on last page, got %s", *page2.NextCursor)
	}
}

func TestListNotifications_InvalidCursor(t *testing.T) {
	svc := newTestNotificationService()
	server := newTestServerWithNotifications(t, svc)
	defer server.Close()

	client := newSessionClient(t, server.URL, 1)
	resp, err := client.Get(server.URL + "/api/v1/notifications?cursor=invalid")
	if err != nil {
		t.Fatalf("GET notifications: %v", err)
	}
	defer closeBody(t, resp.Body)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestNotifications_CrossUserIsolation(t *testing.T) {
	svc := newTestNotificationService()
	cID := int64(1)
	mustCreateNotification(t, svc, 1, "CHAIN", "User 1", "Text", &cID, nil)
	mustCreateNotification(t, svc, 2, "CHAIN", "User 2", "Text", &cID, nil)

	server := newTestServerWithNotifications(t, svc)
	defer server.Close()

	client := newSessionClient(t, server.URL, 1)
	resp, err := client.Get(server.URL + "/api/v1/notifications")
	if err != nil {
		t.Fatalf("GET notifications: %v", err)
	}
	defer closeBody(t, resp.Body)

	var list api.NotificationList
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(list.Notifications) != 1 {
		t.Fatalf("user 1 should see 1 notification, got %d", len(list.Notifications))
	}
	if list.Notifications[0].Title != "User 1" {
		t.Errorf("user 1 should see their own notification")
	}
}

func TestCategoriesListReturnsOnlyUserFacingCategories(t *testing.T) {
	server := newTestServerWithServices(t, &testChainService{chain: testChain()}, items.NewMemoryService())
	defer server.Close()

	client := newSessionClient(t, server.URL, 1)
	response, err := client.Get(server.URL + "/api/v1/categories")
	if err != nil {
		t.Fatalf("GET categories: %v", err)
	}
	defer closeBody(t, response.Body)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET categories status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	var body api.CategoryList
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode categories: %v", err)
	}
	if len(body.Categories) == 0 {
		t.Fatal("expected at least one category")
	}
	for _, c := range body.Categories {
		if c.Id == 47 {
			t.Fatalf("undefined category id=47 was returned to user: %+v", c)
		}
		if c.Name == "" {
			t.Fatalf("category with empty name: %+v", c)
		}
	}
	found := false
	for _, c := range body.Categories {
		if c.IsSystem {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected at least one system category")
	}
}

func TestCategoriesListRequiresSession(t *testing.T) {
	server := newTestServerWithServices(t, &testChainService{chain: testChain()}, items.NewMemoryService())
	defer server.Close()

	response, err := http.Get(server.URL + "/api/v1/categories")
	if err != nil {
		t.Fatalf("GET categories: %v", err)
	}
	defer closeBody(t, response.Body)
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("GET categories status = %d, want %d", response.StatusCode, http.StatusUnauthorized)
	}
}

func TestCreateItemWithCategoryIdStoresAndReturnsIt(t *testing.T) {
	server := newTestServerWithServices(t, &testChainService{chain: testChain()}, items.NewMemoryService())
	defer server.Close()
	client := newSessionClient(t, server.URL, 1)

	payload := `{"offerTitle":"Phone","offerDescription":"Smartphone","wishes": ["Laptop"],"categoryId":1,"condition":"GOOD"}`
	resp := postJSON(t, client, server.URL+"/api/v1/items", payload)
	defer closeBody(t, resp.Body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var created api.Item
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode item: %v", err)
	}
	if created.CategoryId == nil || *created.CategoryId != 1 {
		t.Fatalf("categoryId = %v, want 1", created.CategoryId)
	}
	if created.OfferCategoryId == nil || *created.OfferCategoryId != 1 {
		t.Fatalf("offerCategoryId = %v, want 1", created.OfferCategoryId)
	}
}

func TestCreateItemRejectsUndefinedCategory47(t *testing.T) {
	server := newTestServerWithServices(t, &testChainService{chain: testChain()}, items.NewMemoryService())
	defer server.Close()
	client := newSessionClient(t, server.URL, 1)

	payload := `{"offerTitle":"Phone","offerDescription":"Smartphone","wishes": ["Laptop"],"categoryId":47,"condition":"GOOD"}`
	resp := postJSON(t, client, server.URL+"/api/v1/items", payload)
	defer closeBody(t, resp.Body)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("create status = %d, want %d", resp.StatusCode, http.StatusUnprocessableEntity)
	}
}

func TestCreateItemRejectsInvalidCategoryId(t *testing.T) {
	server := newTestServerWithServices(t, &testChainService{chain: testChain()}, items.NewMemoryService())
	defer server.Close()
	client := newSessionClient(t, server.URL, 1)

	payload := `{"offerTitle":"Phone","offerDescription":"Smartphone","wishes": ["Laptop"],"categoryId":999,"condition":"GOOD"}`
	resp := postJSON(t, client, server.URL+"/api/v1/items", payload)
	defer closeBody(t, resp.Body)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("create status = %d, want %d", resp.StatusCode, http.StatusUnprocessableEntity)
	}
}

func TestUpdateItemChangesCategoryId(t *testing.T) {
	server := newTestServerWithServices(t, &testChainService{chain: testChain()}, items.NewMemoryService())
	defer server.Close()
	client := newSessionClient(t, server.URL, 1)

	createPayload := `{"offerTitle":"Phone","offerDescription":"Smartphone","wishes": ["Laptop"],"categoryId":1,"condition":"GOOD"}`
	createResp := postJSON(t, client, server.URL+"/api/v1/items", createPayload)
	defer closeBody(t, createResp.Body)
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d", createResp.StatusCode)
	}
	var created api.Item
	if err := json.NewDecoder(createResp.Body).Decode(&created); err != nil {
		t.Fatalf("decode item: %v", err)
	}

	updatePayload := `{"categoryId":2}`
	url := fmt.Sprintf("%s/api/v1/items/%d", server.URL, created.Id)
	req, err := http.NewRequest(http.MethodPatch, url, strings.NewReader(updatePayload))
	if err != nil {
		t.Fatalf("create PATCH request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	updateResp, err := client.Do(req)
	if err != nil {
		t.Fatalf("PATCH item: %v", err)
	}
	defer closeBody(t, updateResp.Body)
	if updateResp.StatusCode != http.StatusOK {
		body := readBody(t, updateResp.Body)
		t.Fatalf("PATCH status = %d, want %d; body=%s", updateResp.StatusCode, http.StatusOK, body)
	}
	var updated api.Item
	if err := json.NewDecoder(updateResp.Body).Decode(&updated); err != nil {
		t.Fatalf("decode updated item: %v", err)
	}
	if updated.CategoryId == nil || *updated.CategoryId != 2 {
		t.Fatalf("categoryId = %v, want 2", updated.CategoryId)
	}
}
