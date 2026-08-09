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
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"swap-chain/internal/api"
	"swap-chain/internal/chains"
	"swap-chain/internal/events"
	"swap-chain/internal/httpapi"
	"swap-chain/internal/items"
	"swap-chain/internal/media"
	"swap-chain/internal/session"
	"swap-chain/internal/users"
	"swap-chain/matching/model"
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
		{method: http.MethodPost, path: "/api/v1/items", body: validItemPayload},
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
	if current.User.Id != 7 || current.User.Phone != phoneForUserID(7) {
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
	if current.User.Username != "Danya" || current.User.Phone != "+79995550000" {
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
	var result api.ChainList
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatalf("decode chains: %v", err)
	}
	if len(result.Chains) != 1 || result.Chains[0].Id != 42 {
		t.Fatalf("chains = %#v, want chain 42", result.Chains)
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

	created := postJSON(t, client, server.URL+"/api/v1/items", fmt.Sprintf(`{"offerTitle":"Bike","offerDescription":"Good bike","wantDescription":"Board","imageUrls":[%q]}`, result.Url))
	defer closeBody(t, created.Body)
	if created.StatusCode != http.StatusCreated {
		t.Fatalf("create item status = %d, want %d; body=%s", created.StatusCode, http.StatusCreated, readBody(t, created.Body))
	}

	invalid := postJSON(t, client, server.URL+"/api/v1/items", `{"offerTitle":"Bike","offerDescription":"Good bike","wantDescription":"Board","imageUrls":["/api/v1/media/not-an-object.png"]}`)
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
	handler := httpapi.NewHandler(testDatabase{}, testFinder{}, logger, itemService, mediaService, chainService, eventHub, sessions, userService)
	router, err := New(logger, handler, sessions, "http://localhost:5173", 10<<20)
	if err != nil {
		t.Fatalf("create HTTP handler: %v", err)
	}
	return httptest.NewServer(router)
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
	return users.User{
		ID:        userID,
		Username:  fmt.Sprintf("user-%d", userID),
		Phone:     phoneForUserID(userID),
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

type testChainService struct {
	chain        chains.Chain
	createUserID int64
	createInput  chains.CreateInput
	createErr    error
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
			{User: chains.User{ID: 1, Username: "first"}, GiveItem: first, ReceiveItem: second, Status: chains.ParticipantApproved},
			{User: chains.User{ID: 2, Username: "second"}, GiveItem: second, ReceiveItem: first, Status: chains.ParticipantWaiting},
		},
	}
}

const validItemPayload = `{
  "offerTitle": "Books",
  "offerDescription": "A set of science fiction books",
  "wantDescription": "A strategy board game"
}`

const validChainPayload = `{
  "edges": [
    {"sourceItemId": 10, "targetItemId": 20},
    {"sourceItemId": 20, "targetItemId": 10}
  ]
}`
