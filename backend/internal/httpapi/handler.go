// Package httpapi implements the generated strict HTTP application boundary.
package httpapi

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"

	"swap-chain/internal/api"
	applicationmatching "swap-chain/internal/application/matching"
	"swap-chain/internal/chains"
	"swap-chain/internal/events"
	"swap-chain/internal/items"
	"swap-chain/internal/media"
	"swap-chain/internal/session"
	"swap-chain/internal/users"
	"swap-chain/matching/model"
)

type databasePinger interface {
	PingContext(ctx context.Context) error
}

type readinessChecker interface {
	Ready(ctx context.Context) error
}

type cycleFinder interface {
	Execute(ctx context.Context, itemID int64) ([][]model.Edge, error)
}

type mediaService interface {
	Upload(ctx context.Context, input io.Reader) (media.Object, error)
	Open(ctx context.Context, key string) (media.Object, error)
}

// Handler connects generated HTTP operations to application services.
type Handler struct {
	database  databasePinger
	finder    cycleFinder
	logger    *zap.Logger
	items     items.Service
	media     mediaService
	chains    chains.Service
	events    *events.Hub
	sessions  *session.Manager
	users     users.Service
	readiness []readinessChecker
}

// NewHandler creates the integrated API handler.
func NewHandler(
	database databasePinger,
	finder cycleFinder,
	logger *zap.Logger,
	itemService items.Service,
	mediaService mediaService,
	chainService chains.Service,
	eventHub *events.Hub,
	sessions *session.Manager,
	userService users.Service,
	readiness ...readinessChecker,
) *Handler {
	return &Handler{
		database:  database,
		finder:    finder,
		logger:    logger,
		items:     itemService,
		media:     mediaService,
		chains:    chainService,
		events:    eventHub,
		sessions:  sessions,
		users:     userService,
		readiness: readiness,
	}
}

// UploadMedia stores one image for the current session user.
func (h *Handler) UploadMedia(ctx context.Context, request api.UploadMediaRequestObject) (api.UploadMediaResponseObject, error) {
	if _, ok := session.Current(ctx); !ok {
		return api.UploadMedia401JSONResponse{
			UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(sessionRequired(ctx)),
		}, nil
	}
	if request.Body == nil {
		return uploadMediaBadRequest(ctx, "multipart form body is required"), nil
	}

	for {
		part, err := request.Body.NextPart()
		if errors.Is(err, io.EOF) {
			return uploadMediaBadRequest(ctx, "multipart field file is required"), nil
		}
		if err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				return api.UploadMedia413JSONResponse(errorModel(ctx, "MEDIA_TOO_LARGE", "image exceeds the upload limit", nil)), nil
			}
			return uploadMediaBadRequest(ctx, "invalid multipart form body"), nil
		}

		if part.FormName() != "file" {
			_ = part.Close()
			continue
		}

		stored, uploadErr := h.media.Upload(ctx, part)
		_ = part.Close()
		switch {
		case uploadErr == nil:
			return api.UploadMedia201JSONResponse{
				Url:         media.PublicURL(stored.Key),
				ContentType: api.MediaUploadContentType(stored.ContentType),
				Size:        stored.Size,
			}, nil
		case errors.Is(uploadErr, media.ErrEmpty):
			return uploadMediaBadRequest(ctx, "image file must not be empty"), nil
		case errors.Is(uploadErr, media.ErrTooLarge):
			return api.UploadMedia413JSONResponse(errorModel(ctx, "MEDIA_TOO_LARGE", "image exceeds the upload limit", nil)), nil
		case errors.Is(uploadErr, media.ErrUnsupportedType):
			return api.UploadMedia415JSONResponse(errorModel(ctx, "UNSUPPORTED_MEDIA_TYPE", "only JPEG, PNG, and WebP images are supported", nil)), nil
		default:
			h.logger.Error("upload media", zap.Error(uploadErr))
			return api.UploadMedia500JSONResponse{
				InternalErrorJSONResponse: api.InternalErrorJSONResponse(errorModel(ctx, "INTERNAL_ERROR", "failed to store image", nil)),
			}, nil
		}
	}
}

// GetMedia streams a stored image through the public API origin.
func (h *Handler) GetMedia(ctx context.Context, request api.GetMediaRequestObject) (api.GetMediaResponseObject, error) {
	stored, err := h.media.Open(ctx, request.ObjectKey)
	if errors.Is(err, media.ErrNotFound) {
		return api.GetMedia404JSONResponse{
			NotFoundJSONResponse: api.NotFoundJSONResponse(errorModel(ctx, "MEDIA_NOT_FOUND", "image not found", nil)),
		}, nil
	}
	if err != nil {
		h.logger.Error("get media", zap.String("object_key", request.ObjectKey), zap.Error(err))
		return api.GetMedia500JSONResponse{
			InternalErrorJSONResponse: api.InternalErrorJSONResponse(errorModel(ctx, "INTERNAL_ERROR", "failed to load image", nil)),
		}, nil
	}
	return getMediaResponse{object: stored}, nil
}

type getMediaResponse struct {
	object media.Object
}

func (response getMediaResponse) VisitGetMediaResponse(writer http.ResponseWriter) error {
	writer.Header().Set("Content-Type", response.object.ContentType)
	writer.Header().Set("Content-Length", strconv.FormatInt(response.object.Size, 10))
	writer.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	writer.WriteHeader(http.StatusOK)
	_, copyErr := io.Copy(writer, response.object.Body)
	closeErr := response.object.Body.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func uploadMediaBadRequest(ctx context.Context, message string) api.UploadMedia400JSONResponse {
	return api.UploadMedia400JSONResponse{
		BadRequestJSONResponse: api.BadRequestJSONResponse(errorModel(ctx, "INVALID_MEDIA_UPLOAD", message, nil)),
	}
}

// GetHealth reports readiness of the database and analysis dependencies.
func (h *Handler) GetHealth(ctx context.Context, _ api.GetHealthRequestObject) (api.GetHealthResponseObject, error) {
	response, healthErr := h.health(ctx)
	if healthErr != nil {
		return api.GetHealth503JSONResponse{
			ServiceUnavailableJSONResponse: api.ServiceUnavailableJSONResponse(*healthErr),
		}, nil
	}
	return api.GetHealth200JSONResponse(response), nil
}

// GetLegacyHealth reports process liveness without probing external dependencies.
func (h *Handler) GetLegacyHealth(_ context.Context, _ api.GetLegacyHealthRequestObject) (api.GetLegacyHealthResponseObject, error) {
	return api.GetLegacyHealth200JSONResponse(api.LivenessResponse{
		Status:    api.Alive,
		Timestamp: time.Now().UTC(),
	}), nil
}

func (h *Handler) health(ctx context.Context) (api.HealthResponse, *api.Error) {
	if err := h.database.PingContext(ctx); err != nil {
		h.logger.Error("database health check failed", zap.Error(err))
		response := errorModel(ctx, "DATABASE_UNAVAILABLE", "database is unavailable", nil)
		return api.HealthResponse{}, &response
	}
	for _, checker := range h.readiness {
		if err := checker.Ready(ctx); err != nil {
			h.logger.Error("analysis readiness check failed", zap.Error(err))
			response := errorModel(ctx, "ANALYSIS_UNAVAILABLE", "analysis dependencies are unavailable", nil)
			return api.HealthResponse{}, &response
		}
	}

	return api.HealthResponse{
		Status:    api.Ok,
		Database:  api.Up,
		Analysis:  api.Ready,
		Timestamp: time.Now().UTC(),
	}, nil
}

// GetSession returns the current demo-session identity.
func (h *Handler) GetSession(ctx context.Context, _ api.GetSessionRequestObject) (api.GetSessionResponseObject, error) {
	current, ok := session.Current(ctx)
	if !ok {
		return api.GetSession401JSONResponse{
			UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(sessionRequired(ctx)),
		}, nil
	}
	user, err := h.users.Get(ctx, current.UserID)
	if errors.Is(err, users.ErrNotFound) {
		return api.GetSession401JSONResponse{
			UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(sessionRequired(ctx)),
		}, nil
	}
	if err != nil {
		h.logger.Error("get current user", zap.Error(err))
		return api.GetSession500JSONResponse{
			InternalErrorJSONResponse: api.InternalErrorJSONResponse(errorModel(ctx, "INTERNAL_ERROR", "failed to load current user", nil)),
		}, nil
	}
	return api.GetSession200JSONResponse(sessionModel(current, user)), nil
}

// Login identifies a registered demo user by phone and issues an opaque cookie.
func (h *Handler) Login(ctx context.Context, request api.LoginRequestObject) (api.LoginResponseObject, error) {
	if request.Body == nil {
		return api.Login400JSONResponse{
			BadRequestJSONResponse: api.BadRequestJSONResponse(errorModel(ctx, "INVALID_REQUEST", "JSON request body is required", nil)),
		}, nil
	}

	user, err := h.users.FindByPhone(ctx, request.Body.Phone)
	if response := loginUserError(ctx, err); response != nil {
		return response, nil
	}
	created, cookie, err := h.sessions.Create(user.ID)
	if err != nil {
		h.logger.Error("create session", zap.Error(err))
		return api.Login500JSONResponse{
			InternalErrorJSONResponse: api.InternalErrorJSONResponse(errorModel(ctx, "INTERNAL_ERROR", "failed to create demo session", nil)),
		}, nil
	}

	return api.Login200JSONResponse{
		Body: sessionModel(created, user),
		Headers: api.Login200ResponseHeaders{
			SetCookie: &cookie,
		},
	}, nil
}

// Logout revokes the current opaque session and clears its cookie.
func (h *Handler) Logout(ctx context.Context, _ api.LogoutRequestObject) (api.LogoutResponseObject, error) {
	current, ok := session.Current(ctx)
	if !ok {
		return api.Logout401JSONResponse{
			UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(sessionRequired(ctx)),
		}, nil
	}
	cookie := h.sessions.Revoke(current)
	return api.Logout204Response{Headers: api.Logout204ResponseHeaders{SetCookie: &cookie}}, nil
}

// CreateUser registers a demo user and establishes their first session.
func (h *Handler) CreateUser(ctx context.Context, request api.CreateUserRequestObject) (api.CreateUserResponseObject, error) {
	if request.Body == nil {
		return api.CreateUser400JSONResponse{
			BadRequestJSONResponse: api.BadRequestJSONResponse(errorModel(ctx, "INVALID_REQUEST", "JSON request body is required", nil)),
		}, nil
	}

	user, err := h.users.Create(ctx, users.CreateInput{Username: request.Body.Username, Phone: request.Body.Phone})
	if response := createUserError(ctx, err); response != nil {
		return response, nil
	}
	created, cookie, err := h.sessions.Create(user.ID)
	if err != nil {
		h.logger.Error("create registration session", zap.Error(err))
		return api.CreateUser500JSONResponse{
			InternalErrorJSONResponse: api.InternalErrorJSONResponse(errorModel(ctx, "INTERNAL_ERROR", "failed to create demo session", nil)),
		}, nil
	}
	return api.CreateUser201JSONResponse{
		Body: sessionModel(created, user),
		Headers: api.CreateUser201ResponseHeaders{
			SetCookie: &cookie,
		},
	}, nil
}

// GetUser exposes the generated contract until the user service is connected.
func (h *Handler) GetUser(ctx context.Context, _ api.GetUserRequestObject) (api.GetUserResponseObject, error) {
	return api.GetUser501JSONResponse{
		NotImplementedJSONResponse: api.NotImplementedJSONResponse(userServiceNotImplemented(ctx)),
	}, nil
}

// ListUserItems exposes the generated contract until the user service is connected.
func (h *Handler) ListUserItems(ctx context.Context, _ api.ListUserItemsRequestObject) (api.ListUserItemsResponseObject, error) {
	return api.ListUserItems501JSONResponse{
		NotImplementedJSONResponse: api.NotImplementedJSONResponse(userServiceNotImplemented(ctx)),
	}, nil
}

// ListItems returns a cursor-paginated item list.
func (h *Handler) ListItems(ctx context.Context, request api.ListItemsRequestObject) (api.ListItemsResponseObject, error) {
	current, ok := session.Current(ctx)
	if !ok {
		return api.ListItems401JSONResponse{
			UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(sessionRequired(ctx)),
		}, nil
	}
	limit := 20
	if request.Params.Limit != nil {
		limit = *request.Params.Limit
	}

	cursor := int64(0)
	if request.Params.Cursor != nil {
		parsed, err := items.ParseCursor(*request.Params.Cursor)
		if err != nil {
			return api.ListItems400JSONResponse{
				BadRequestJSONResponse: api.BadRequestJSONResponse(errorModel(ctx, "INVALID_CURSOR", "cursor must be a non-negative integer", nil)),
			}, nil
		}
		cursor = parsed
	}

	result, next, err := h.items.ListByUser(ctx, current.UserID, cursor, limit)
	if err != nil {
		return api.ListItems500JSONResponse{
			InternalErrorJSONResponse: api.InternalErrorJSONResponse(errorModel(ctx, "INTERNAL_ERROR", "failed to list items", nil)),
		}, nil
	}

	response := api.ItemList{Items: make([]api.Item, 0, len(result))}
	for _, item := range result {
		response.Items = append(response.Items, itemModel(item))
	}
	if next != nil {
		value := items.FormatCursor(*next)
		response.NextCursor = &value
	}

	return api.ListItems200JSONResponse(response), nil
}

// CreateItem creates an item for the current demo-session user.
func (h *Handler) CreateItem(ctx context.Context, request api.CreateItemRequestObject) (api.CreateItemResponseObject, error) {
	current, ok := session.Current(ctx)
	if !ok {
		return api.CreateItem401JSONResponse{
			UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(sessionRequired(ctx)),
		}, nil
	}
	if request.Body == nil {
		return api.CreateItem400JSONResponse{
			BadRequestJSONResponse: api.BadRequestJSONResponse(errorModel(ctx, "INVALID_REQUEST", "JSON request body is required", nil)),
		}, nil
	}

	imageURLs := make([]string, 0)
	if request.Body.ImageUrls != nil {
		imageURLs = append(imageURLs, (*request.Body.ImageUrls)...)
	}

	item, err := h.items.Create(ctx, current.UserID, items.CreateInput{
		OfferTitle:       request.Body.OfferTitle,
		OfferDescription: request.Body.OfferDescription,
		WantDescription:  request.Body.WantDescription,
		ImageURLs:        imageURLs,
	})
	if err != nil {
		var validationError *items.ValidationError
		if errors.As(err, &validationError) {
			details := make(map[string]any, len(validationError.Fields))
			for field, message := range validationError.Fields {
				details[field] = message
			}
			return api.CreateItem422JSONResponse{
				ValidationErrorJSONResponse: api.ValidationErrorJSONResponse(errorModel(ctx, "VALIDATION_ERROR", "item validation failed", details)),
			}, nil
		}
		return api.CreateItem500JSONResponse{
			InternalErrorJSONResponse: api.InternalErrorJSONResponse(errorModel(ctx, "INTERNAL_ERROR", "failed to create item", nil)),
		}, nil
	}

	return api.CreateItem201JSONResponse(itemModel(item)), nil
}

// GetItem returns one item.
func (h *Handler) GetItem(ctx context.Context, request api.GetItemRequestObject) (api.GetItemResponseObject, error) {
	item, err := h.items.Get(ctx, request.ItemId)
	if errors.Is(err, items.ErrNotFound) {
		return api.GetItem404JSONResponse{
			NotFoundJSONResponse: api.NotFoundJSONResponse(errorModel(ctx, "ITEM_NOT_FOUND", "item not found", nil)),
		}, nil
	}
	if err != nil {
		return api.GetItem500JSONResponse{
			InternalErrorJSONResponse: api.InternalErrorJSONResponse(errorModel(ctx, "INTERNAL_ERROR", "failed to get item", nil)),
		}, nil
	}

	return api.GetItem200JSONResponse(itemModel(item)), nil
}

// FindMatchingCycles runs the integrated matching use case.
func (h *Handler) FindMatchingCycles(ctx context.Context, request api.FindMatchingCyclesRequestObject) (api.FindMatchingCyclesResponseObject, error) {
	current, ok := session.Current(ctx)
	if !ok {
		return api.FindMatchingCycles401JSONResponse{
			UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(sessionRequired(ctx)),
		}, nil
	}
	item, err := h.items.Get(ctx, request.ItemId)
	if errors.Is(err, items.ErrNotFound) {
		return api.FindMatchingCycles404JSONResponse{
			NotFoundJSONResponse: api.NotFoundJSONResponse(errorModel(ctx, "ITEM_NOT_FOUND", "item not found", nil)),
		}, nil
	}
	if err != nil {
		h.logger.Error("load matching root item", zap.Int64("item_id", request.ItemId), zap.Error(err))
		return api.FindMatchingCycles500JSONResponse{
			InternalErrorJSONResponse: api.InternalErrorJSONResponse(errorModel(ctx, "MATCHING_FAILED", "matching could not be completed", nil)),
		}, nil
	}
	if item.UserID != current.UserID {
		return api.FindMatchingCycles403JSONResponse{
			ForbiddenJSONResponse: api.ForbiddenJSONResponse(errorModel(ctx, "FORBIDDEN", "item is not available to the current user", nil)),
		}, nil
	}
	if item.Status != "MATCHING" {
		return api.FindMatchingCycles409JSONResponse{
			ConflictJSONResponse: api.ConflictJSONResponse(errorModel(ctx, "ITEM_NOT_READY", "item is not ready for matching", map[string]any{"status": item.Status})),
		}, nil
	}

	cycles, err := h.finder.Execute(ctx, request.ItemId)
	if err != nil {
		if errors.Is(err, applicationmatching.ErrInvalidItemID) {
			return api.FindMatchingCycles400JSONResponse{
				BadRequestJSONResponse: api.BadRequestJSONResponse(errorModel(ctx, "INVALID_ITEM_ID", err.Error(), nil)),
			}, nil
		}

		h.logger.Error("find matching cycles", zap.Int64("item_id", request.ItemId), zap.Error(err))
		return api.FindMatchingCycles500JSONResponse{
			InternalErrorJSONResponse: api.InternalErrorJSONResponse(errorModel(ctx, "MATCHING_FAILED", "matching could not be completed", nil)),
		}, nil
	}

	response := api.MatchingResponse{ItemId: request.ItemId, Cycles: make([]api.MatchingCycle, 0, len(cycles))}
	for _, cycle := range cycles {
		edges := make([]api.MatchingEdge, 0, len(cycle))
		for _, edge := range cycle {
			edges = append(edges, api.MatchingEdge{
				SourceItemId: int64(edge.SourceID),
				TargetItemId: int64(edge.TargetID),
				Score:        float32(edge.Score),
			})
		}
		response.Cycles = append(response.Cycles, api.MatchingCycle{Edges: edges})
	}

	return api.FindMatchingCycles200JSONResponse(response), nil
}

// ListChains returns cursor-paginated proposals involving the current user.
func (h *Handler) ListChains(ctx context.Context, request api.ListChainsRequestObject) (api.ListChainsResponseObject, error) {
	current, ok := session.Current(ctx)
	if !ok {
		return api.ListChains401JSONResponse{
			UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(sessionRequired(ctx)),
		}, nil
	}

	limit := 20
	if request.Params.Limit != nil {
		limit = *request.Params.Limit
	}
	status := ""
	if request.Params.Status != nil {
		status = string(*request.Params.Status)
	}
	afterID := int64(0)
	if request.Params.Cursor != nil {
		parsed, err := strconv.ParseInt(*request.Params.Cursor, 10, 64)
		if err != nil || parsed < 0 {
			return api.ListChains400JSONResponse{
				BadRequestJSONResponse: api.BadRequestJSONResponse(errorModel(ctx, "INVALID_CURSOR", "cursor must be a non-negative integer", nil)),
			}, nil
		}
		afterID = parsed
	}

	result, next, err := h.chains.List(ctx, current.UserID, status, afterID, limit)
	if err != nil {
		if isChainValidationError(err) {
			return api.ListChains400JSONResponse{
				BadRequestJSONResponse: api.BadRequestJSONResponse(errorModel(ctx, "VALIDATION_ERROR", err.Error(), nil)),
			}, nil
		}
		h.logger.Error("list chains", zap.Error(err))
		return api.ListChains500JSONResponse{
			InternalErrorJSONResponse: api.InternalErrorJSONResponse(chainInternalError(ctx)),
		}, nil
	}

	response := api.ChainList{Chains: make([]api.Chain, 0, len(result))}
	for _, chain := range result {
		response.Chains = append(response.Chains, chainModel(chain))
	}
	if next != nil {
		cursor := strconv.FormatInt(*next, 10)
		response.NextCursor = &cursor
	}
	return api.ListChains200JSONResponse(response), nil
}

// CreateChain persists a selected matching cycle as a pending proposal.
func (h *Handler) CreateChain(ctx context.Context, request api.CreateChainRequestObject) (api.CreateChainResponseObject, error) {
	current, ok := session.Current(ctx)
	if !ok {
		return api.CreateChain401JSONResponse{
			UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(sessionRequired(ctx)),
		}, nil
	}
	if request.Body == nil {
		return api.CreateChain400JSONResponse{
			ValidationErrorJSONResponse: api.ValidationErrorJSONResponse(errorModel(ctx, "INVALID_REQUEST", "JSON request body is required", nil)),
		}, nil
	}

	edges := make([]chains.Edge, 0, len(request.Body.Edges))
	for _, edge := range request.Body.Edges {
		edges = append(edges, chains.Edge{SourceItemID: edge.SourceItemId, TargetItemID: edge.TargetItemId})
	}
	created, err := h.chains.Create(ctx, current.UserID, chains.CreateInput{Edges: edges})
	if err != nil {
		model := chainErrorModel(ctx, err)
		switch {
		case isChainValidationError(err):
			return api.CreateChain400JSONResponse{ValidationErrorJSONResponse: api.ValidationErrorJSONResponse(model)}, nil
		case errors.Is(err, chains.ErrForbidden):
			return api.CreateChain403JSONResponse{ForbiddenJSONResponse: api.ForbiddenJSONResponse(model)}, nil
		case errors.Is(err, chains.ErrNotFound):
			return api.CreateChain404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse(model)}, nil
		case errors.Is(err, chains.ErrConflict):
			return api.CreateChain409JSONResponse{ConflictJSONResponse: api.ConflictJSONResponse(model)}, nil
		default:
			h.logger.Error("create chain", zap.Error(err))
			return api.CreateChain500JSONResponse{InternalErrorJSONResponse: api.InternalErrorJSONResponse(model)}, nil
		}
	}
	return api.CreateChain201JSONResponse(chainModel(created)), nil
}

// GetChain returns one proposal involving the current user.
func (h *Handler) GetChain(ctx context.Context, request api.GetChainRequestObject) (api.GetChainResponseObject, error) {
	current, ok := session.Current(ctx)
	if !ok {
		return api.GetChain401JSONResponse{
			UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(sessionRequired(ctx)),
		}, nil
	}
	chain, err := h.chains.Get(ctx, current.UserID, request.ChainId)
	if err != nil {
		model := chainErrorModel(ctx, err)
		switch {
		case errors.Is(err, chains.ErrForbidden):
			return api.GetChain403JSONResponse{ForbiddenJSONResponse: api.ForbiddenJSONResponse(model)}, nil
		case errors.Is(err, chains.ErrNotFound):
			return api.GetChain404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse(model)}, nil
		default:
			h.logger.Error("get chain", zap.Error(err))
			return api.GetChain500JSONResponse{InternalErrorJSONResponse: api.InternalErrorJSONResponse(model)}, nil
		}
	}
	return api.GetChain200JSONResponse(chainModel(chain)), nil
}

// SubmitChainDecision records the current participant's decision.
func (h *Handler) SubmitChainDecision(ctx context.Context, request api.SubmitChainDecisionRequestObject) (api.SubmitChainDecisionResponseObject, error) {
	current, ok := session.Current(ctx)
	if !ok {
		return api.SubmitChainDecision401JSONResponse{
			UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(sessionRequired(ctx)),
		}, nil
	}
	if request.Body == nil {
		return api.SubmitChainDecision400JSONResponse{
			BadRequestJSONResponse: api.BadRequestJSONResponse(errorModel(ctx, "INVALID_REQUEST", "JSON request body is required", nil)),
		}, nil
	}

	chain, err := h.chains.Decide(ctx, current.UserID, request.ChainId, string(request.Body.Decision))
	if err != nil {
		model := chainErrorModel(ctx, err)
		switch {
		case isChainValidationError(err):
			return api.SubmitChainDecision400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse(model)}, nil
		case errors.Is(err, chains.ErrForbidden):
			return api.SubmitChainDecision403JSONResponse{ForbiddenJSONResponse: api.ForbiddenJSONResponse(model)}, nil
		case errors.Is(err, chains.ErrNotFound):
			return api.SubmitChainDecision404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse(model)}, nil
		case errors.Is(err, chains.ErrConflict):
			return api.SubmitChainDecision409JSONResponse{ConflictJSONResponse: api.ConflictJSONResponse(model)}, nil
		default:
			h.logger.Error("submit chain decision", zap.Error(err))
			return api.SubmitChainDecision500JSONResponse{InternalErrorJSONResponse: api.InternalErrorJSONResponse(model)}, nil
		}
	}
	return api.SubmitChainDecision200JSONResponse(chainModel(chain)), nil
}

// SubscribeEvents streams only events addressed to the current demo user.
func (h *Handler) SubscribeEvents(ctx context.Context, _ api.SubscribeEventsRequestObject) (api.SubscribeEventsResponseObject, error) {
	current, ok := session.Current(ctx)
	if !ok {
		return api.SubscribeEvents401JSONResponse{
			UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(sessionRequired(ctx)),
		}, nil
	}

	cacheControl := "no-cache"
	connection := "keep-alive"
	return api.SubscribeEvents200TexteventStreamResponse{
		Body: events.NewStream(ctx, h.events, current.UserID),
		Headers: api.SubscribeEvents200ResponseHeaders{
			CacheControl: &cacheControl,
			Connection:   &connection,
		},
	}, nil
}

func itemModel(item items.Item) api.Item {
	return api.Item{
		Id:               item.ID,
		UserId:           item.UserID,
		OfferTitle:       item.OfferTitle,
		OfferDescription: item.OfferDescription,
		WantDescription:  item.WantDescription,
		ImageUrls:        append([]string(nil), item.ImageURLs...),
		Status:           api.ItemStatus(item.Status),
		CreatedAt:        item.CreatedAt,
		UpdatedAt:        item.UpdatedAt,
	}
}

func chainModel(chain chains.Chain) api.Chain {
	participants := make([]api.ChainParticipant, 0, len(chain.Participants))
	for _, participant := range chain.Participants {
		participants = append(participants, api.ChainParticipant{
			User: api.UserSummary{
				Id:       participant.User.ID,
				Username: participant.User.Username,
			},
			GiveItem:    itemModel(participant.GiveItem),
			ReceiveItem: itemModel(participant.ReceiveItem),
			Status:      api.ParticipantStatus(participant.Status),
		})
	}
	return api.Chain{
		Id:           chain.ID,
		Status:       api.ChainStatus(chain.Status),
		Participants: participants,
		CreatedAt:    chain.CreatedAt,
		ExpiresAt:    chain.ExpiresAt,
	}
}

func sessionModel(current session.Session, user users.User) api.Session {
	return api.Session{
		ExpiresAt: current.ExpiresAt,
		User: api.CurrentUser{
			Id:        user.ID,
			Username:  user.Username,
			Phone:     user.Phone,
			CreatedAt: user.CreatedAt,
		},
	}
}

func errorModel(ctx context.Context, code, message string, details map[string]any) api.Error {
	requestID := middleware.GetReqID(ctx)
	model := api.Error{Code: code, Message: message}
	if requestID != "" {
		model.RequestId = &requestID
	}
	if len(details) > 0 {
		model.Details = &details
	}
	return model
}

func sessionRequired(ctx context.Context) api.Error {
	return errorModel(ctx, "SESSION_REQUIRED", "establish a demo session first", nil)
}

func loginUserError(ctx context.Context, err error) api.LoginResponseObject {
	if err == nil {
		return nil
	}
	var validationError *users.ValidationError
	if errors.As(err, &validationError) {
		return api.Login400JSONResponse{
			BadRequestJSONResponse: api.BadRequestJSONResponse(userValidationError(ctx, validationError)),
		}
	}
	if errors.Is(err, users.ErrNotFound) {
		return api.Login404JSONResponse{
			NotFoundJSONResponse: api.NotFoundJSONResponse(errorModel(ctx, "USER_NOT_FOUND", "no user is registered with this phone", nil)),
		}
	}
	return api.Login500JSONResponse{
		InternalErrorJSONResponse: api.InternalErrorJSONResponse(errorModel(ctx, "INTERNAL_ERROR", "failed to log in", nil)),
	}
}

func createUserError(ctx context.Context, err error) api.CreateUserResponseObject {
	if err == nil {
		return nil
	}
	var validationError *users.ValidationError
	if errors.As(err, &validationError) {
		return api.CreateUser400JSONResponse{
			BadRequestJSONResponse: api.BadRequestJSONResponse(userValidationError(ctx, validationError)),
		}
	}
	if errors.Is(err, users.ErrPhoneExists) {
		return api.CreateUser409JSONResponse{
			ConflictJSONResponse: api.ConflictJSONResponse(errorModel(ctx, "PHONE_ALREADY_REGISTERED", "this phone is already registered", nil)),
		}
	}
	return api.CreateUser500JSONResponse{
		InternalErrorJSONResponse: api.InternalErrorJSONResponse(errorModel(ctx, "INTERNAL_ERROR", "failed to register user", nil)),
	}
}

func userValidationError(ctx context.Context, err *users.ValidationError) api.Error {
	return errorModel(ctx, "VALIDATION_ERROR", err.Error(), map[string]any{"fields": err.Fields})
}

func isChainValidationError(err error) bool {
	var validationError *chains.ValidationError
	return errors.As(err, &validationError)
}

func chainErrorModel(ctx context.Context, err error) api.Error {
	switch {
	case isChainValidationError(err):
		return errorModel(ctx, "VALIDATION_ERROR", err.Error(), nil)
	case errors.Is(err, chains.ErrForbidden):
		return errorModel(ctx, "FORBIDDEN", "chain is not available to the current user", nil)
	case errors.Is(err, chains.ErrNotFound):
		return errorModel(ctx, "NOT_FOUND", "chain or item was not found", nil)
	case errors.Is(err, chains.ErrConflict):
		return errorModel(ctx, "CHAIN_CONFLICT", "chain conflicts with the current item or chain state", nil)
	default:
		return chainInternalError(ctx)
	}
}

func chainInternalError(ctx context.Context) api.Error {
	return errorModel(ctx, "INTERNAL_ERROR", "chain operation failed", nil)
}

func userServiceNotImplemented(ctx context.Context) api.Error {
	return errorModel(ctx, "USER_SERVICE_NOT_IMPLEMENTED", "user business service is not connected", nil)
}
