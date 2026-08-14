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
	"swap-chain/internal/categories"
	"swap-chain/internal/chains"
	"swap-chain/internal/events"
	"swap-chain/internal/items"
	"swap-chain/internal/media"
	"swap-chain/internal/session"
	"swap-chain/internal/users"
	adminmodel "swap-chain/modules/admin/model"
	adminservice "swap-chain/modules/admin/service"
	blocklistmodel "swap-chain/modules/blocklist/model"
	blocklistservice "swap-chain/modules/blocklist/service"
	chatmodel "swap-chain/modules/chat/model"
	chatservice "swap-chain/modules/chat/service"
	"swap-chain/modules/matching/model"
	metricsmodel "swap-chain/modules/metrics/model"
	metricsservice "swap-chain/modules/metrics/service"
	moderationmodel "swap-chain/modules/moderation/model"
	moderationservice "swap-chain/modules/moderation/service"
	notificationmodel "swap-chain/modules/notifications/model"
	notificationservice "swap-chain/modules/notifications/service"
	reputationmodel "swap-chain/modules/reputation/model"
	reputationservice "swap-chain/modules/reputation/service"
	supportservice "swap-chain/modules/support/service"
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
	database      databasePinger
	finder        cycleFinder
	logger        *zap.Logger
	items         items.Service
	media         mediaService
	chains        chains.Service
	categories    categories.Service
	events        *events.Hub
	sessions      *session.Manager
	users         users.Service
	admin         adminservice.Service
	chat          chatservice.Service
	support       *supportservice.Chat
	blocklist     blocklistservice.Service
	moderation    moderationservice.Service
	notifications notificationservice.Service
	reputation    reputationservice.Service
	metrics       metricsservice.Service
	vision        VisionService
	visionJobs    chan struct{}
	readiness     []readinessChecker
}

// NewHandler creates the integrated API handler.
func NewHandler(
	database databasePinger,
	finder cycleFinder,
	logger *zap.Logger,
	itemService items.Service,
	mediaService mediaService,
	chainService chains.Service,
	categoriesService categories.Service,
	eventHub *events.Hub,
	sessions *session.Manager,
	userService users.Service,
	adminService adminservice.Service,
	chatService chatservice.Service,
	supportService *supportservice.Chat,
	blocklistService blocklistservice.Service,
	moderationService moderationservice.Service,
	notificationService notificationservice.Service,
	reputationService reputationservice.Service,
	metricsService metricsservice.Service,
	visionService VisionService,
	readiness ...readinessChecker,
) *Handler {
	handler := &Handler{
		database:      database,
		finder:        finder,
		logger:        logger,
		items:         itemService,
		media:         mediaService,
		chains:        chainService,
		categories:    categoriesService,
		events:        eventHub,
		sessions:      sessions,
		users:         userService,
		admin:         adminService,
		chat:          chatService,
		support:       supportService,
		blocklist:     blocklistService,
		moderation:    moderationService,
		notifications: notificationService,
		reputation:    reputationService,
		metrics:       metricsService,
		vision:        visionService,
		readiness:     readiness,
	}
	if visionService != nil {
		handler.visionJobs = make(chan struct{}, maxPendingVisionJobs)
	}
	return handler
}

// GetAdminFunnelMetrics returns an internally consistent product funnel snapshot.
func (h *Handler) GetAdminFunnelMetrics(ctx context.Context, _ api.GetAdminFunnelMetricsRequestObject) (api.GetAdminFunnelMetricsResponseObject, error) {
	current, ok := session.Current(ctx)
	if !ok {
		return api.GetAdminFunnelMetrics401JSONResponse{
			UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(sessionRequired(ctx)),
		}, nil
	}

	metrics, err := h.metrics.Funnel(ctx, current.UserID)
	if errors.Is(err, metricsmodel.ErrForbidden) {
		return api.GetAdminFunnelMetrics403JSONResponse{
			ForbiddenJSONResponse: api.ForbiddenJSONResponse(errorModel(ctx, "ADMIN_REQUIRED", "admin access is required", nil)),
		}, nil
	}
	if err != nil {
		h.logger.Error("get admin funnel metrics", zap.Int64("actor_id", current.UserID), zap.Error(err))
		return api.GetAdminFunnelMetrics500JSONResponse{
			InternalErrorJSONResponse: api.InternalErrorJSONResponse(errorModel(ctx, "INTERNAL_ERROR", "failed to load product metrics", nil)),
		}, nil
	}
	return api.GetAdminFunnelMetrics200JSONResponse(funnelMetricsModel(metrics)), nil
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

// GetUser returns a user profile by ID.
func (h *Handler) GetUser(ctx context.Context, request api.GetUserRequestObject) (api.GetUserResponseObject, error) {
	user, err := h.users.Get(ctx, request.UserId)
	if errors.Is(err, users.ErrNotFound) {
		return api.GetUser404JSONResponse{
			NotFoundJSONResponse: api.NotFoundJSONResponse(errorModel(ctx, "USER_NOT_FOUND", "user not found", nil)),
		}, nil
	}
	if err != nil {
		h.logger.Error("get user", zap.Int64("user_id", request.UserId), zap.Error(err))
		return api.GetUser500JSONResponse{
			InternalErrorJSONResponse: api.InternalErrorJSONResponse(errorModel(ctx, "INTERNAL_ERROR", "failed to load user profile", nil)),
		}, nil
	}
	stats, err := h.reputation.Stats(ctx, user.ID)
	if err != nil {
		h.logger.Error("get user reputation", zap.Int64("user_id", user.ID), zap.Error(err))
		return api.GetUser500JSONResponse{
			InternalErrorJSONResponse: api.InternalErrorJSONResponse(errorModel(ctx, "INTERNAL_ERROR", "failed to load user profile", nil)),
		}, nil
	}
	return api.GetUser200JSONResponse(userProfileModel(user, stats)), nil
}

// UpdateUser updates the profile of the currently authenticated user.
func (h *Handler) UpdateUser(ctx context.Context, request api.UpdateUserRequestObject) (api.UpdateUserResponseObject, error) {
	current, ok := session.Current(ctx)
	if !ok {
		return api.UpdateUser401JSONResponse{
			UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(sessionRequired(ctx)),
		}, nil
	}
	if request.UserId != current.UserID {
		return api.UpdateUser403JSONResponse{
			ForbiddenJSONResponse: api.ForbiddenJSONResponse(errorModel(ctx, "USER_FORBIDDEN", "profile can only be edited by its owner", nil)),
		}, nil
	}
	if request.Body == nil {
		return api.UpdateUser400JSONResponse{
			BadRequestJSONResponse: api.BadRequestJSONResponse(errorModel(ctx, "INVALID_REQUEST", "JSON request body is required", nil)),
		}, nil
	}

	user, err := h.users.Update(ctx, current.UserID, users.UpdateInput{Username: request.Body.Username, AvatarURL: request.Body.AvatarUrl})
	if err != nil {
		var validationError *users.ValidationError
		switch {
		case errors.As(err, &validationError):
			details := make(map[string]any, len(validationError.Fields))
			for field, message := range validationError.Fields {
				details[field] = message
			}
			return api.UpdateUser400JSONResponse{
				BadRequestJSONResponse: api.BadRequestJSONResponse(errorModel(ctx, "VALIDATION_ERROR", "user validation failed", details)),
			}, nil
		case errors.Is(err, users.ErrNotFound):
			return api.UpdateUser404JSONResponse{
				NotFoundJSONResponse: api.NotFoundJSONResponse(errorModel(ctx, "USER_NOT_FOUND", "user not found", nil)),
			}, nil
		default:
			h.logger.Error("update user", zap.Int64("user_id", current.UserID), zap.Error(err))
			return api.UpdateUser500JSONResponse{
				InternalErrorJSONResponse: api.InternalErrorJSONResponse(errorModel(ctx, "INTERNAL_ERROR", "failed to update user profile", nil)),
			}, nil
		}
	}
	stats, err := h.reputation.Stats(ctx, user.ID)
	if err != nil {
		h.logger.Error("get updated user reputation", zap.Int64("user_id", user.ID), zap.Error(err))
		return api.UpdateUser500JSONResponse{
			InternalErrorJSONResponse: api.InternalErrorJSONResponse(errorModel(ctx, "INTERNAL_ERROR", "failed to load updated user profile", nil)),
		}, nil
	}
	return api.UpdateUser200JSONResponse(userProfileModel(user, stats)), nil
}

// ListUserReviews returns reviews received by the requested user.
func (h *Handler) ListUserReviews(ctx context.Context, request api.ListUserReviewsRequestObject) (api.ListUserReviewsResponseObject, error) {
	limit := 20
	if request.Params.Limit != nil {
		limit = *request.Params.Limit
	}
	cursor := int64(0)
	if request.Params.Cursor != nil {
		parsed, err := strconv.ParseInt(*request.Params.Cursor, 10, 64)
		if err != nil || parsed < 0 {
			return api.ListUserReviews400JSONResponse{
				BadRequestJSONResponse: api.BadRequestJSONResponse(errorModel(ctx, "INVALID_CURSOR", "cursor must be a non-negative integer", nil)),
			}, nil
		}
		cursor = parsed
	}

	result, err := h.reputation.ListReviews(ctx, request.UserId, cursor, limit)
	if err != nil {
		var validationError *reputationmodel.ValidationError
		switch {
		case errors.As(err, &validationError):
			return api.ListUserReviews400JSONResponse{
				BadRequestJSONResponse: api.BadRequestJSONResponse(errorModel(ctx, "VALIDATION_ERROR", validationError.Error(), map[string]any{validationError.Field: validationError.Message})),
			}, nil
		case errors.Is(err, reputationmodel.ErrUserNotFound):
			return api.ListUserReviews404JSONResponse{
				NotFoundJSONResponse: api.NotFoundJSONResponse(errorModel(ctx, "USER_NOT_FOUND", "user not found", nil)),
			}, nil
		default:
			h.logger.Error("list user reviews", zap.Int64("user_id", request.UserId), zap.Error(err))
			return api.ListUserReviews500JSONResponse{
				InternalErrorJSONResponse: api.InternalErrorJSONResponse(errorModel(ctx, "INTERNAL_ERROR", "failed to list user reviews", nil)),
			}, nil
		}
	}

	response := api.UserReviewList{Reviews: make([]api.UserReview, 0, len(result.Reviews))}
	for _, review := range result.Reviews {
		response.Reviews = append(response.Reviews, userReviewModel(review))
	}
	if result.NextCursor != nil {
		next := strconv.FormatInt(*result.NextCursor, 10)
		response.NextCursor = &next
	}
	return api.ListUserReviews200JSONResponse(response), nil
}

// CreateChainReview creates one immutable review for a direct neighbour in a completed chain.
func (h *Handler) CreateChainReview(ctx context.Context, request api.CreateChainReviewRequestObject) (api.CreateChainReviewResponseObject, error) {
	current, ok := session.Current(ctx)
	if !ok {
		return api.CreateChainReview401JSONResponse{
			UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(sessionRequired(ctx)),
		}, nil
	}
	if request.Body == nil {
		return api.CreateChainReview400JSONResponse{
			ValidationErrorJSONResponse: api.ValidationErrorJSONResponse(errorModel(ctx, "INVALID_REQUEST", "JSON request body is required", nil)),
		}, nil
	}

	review, err := h.reputation.CreateReview(ctx, current.UserID, request.ChainId, reputationmodel.CreateInput{
		TargetUserID: request.Body.TargetUserId,
		Rating:       request.Body.Rating,
		Text:         request.Body.Text,
	})
	if err != nil {
		var validationError *reputationmodel.ValidationError
		switch {
		case errors.As(err, &validationError):
			return api.CreateChainReview400JSONResponse{
				ValidationErrorJSONResponse: api.ValidationErrorJSONResponse(errorModel(ctx, "VALIDATION_ERROR", validationError.Error(), map[string]any{validationError.Field: validationError.Message})),
			}, nil
		case errors.Is(err, reputationmodel.ErrForbidden):
			return api.CreateChainReview403JSONResponse{
				ForbiddenJSONResponse: api.ForbiddenJSONResponse(errorModel(ctx, "REVIEW_FORBIDDEN", "only a direct exchange neighbour can be reviewed", nil)),
			}, nil
		case errors.Is(err, reputationmodel.ErrChainNotFound), errors.Is(err, reputationmodel.ErrUserNotFound):
			return api.CreateChainReview404JSONResponse{
				NotFoundJSONResponse: api.NotFoundJSONResponse(errorModel(ctx, "REVIEW_TARGET_NOT_FOUND", "chain or review target not found", nil)),
			}, nil
		case errors.Is(err, reputationmodel.ErrChainNotCompleted):
			return api.CreateChainReview409JSONResponse{
				ConflictJSONResponse: api.ConflictJSONResponse(errorModel(ctx, "CHAIN_NOT_COMPLETED", "reviews are available only after the exchange is completed", nil)),
			}, nil
		case errors.Is(err, reputationmodel.ErrAlreadyExists):
			return api.CreateChainReview409JSONResponse{
				ConflictJSONResponse: api.ConflictJSONResponse(errorModel(ctx, "REVIEW_ALREADY_EXISTS", "this participant was already reviewed for the chain", nil)),
			}, nil
		default:
			h.logger.Error("create chain review", zap.Int64("chain_id", request.ChainId), zap.Int64("author_user_id", current.UserID), zap.Error(err))
			return api.CreateChainReview500JSONResponse{
				InternalErrorJSONResponse: api.InternalErrorJSONResponse(errorModel(ctx, "INTERNAL_ERROR", "failed to create review", nil)),
			}, nil
		}
	}
	return api.CreateChainReview201JSONResponse(userReviewModel(review)), nil
}

// ListUserItems exposes the generated contract until the user service is connected.
func (h *Handler) ListUserItems(ctx context.Context, _ api.ListUserItemsRequestObject) (api.ListUserItemsResponseObject, error) {
	return api.ListUserItems501JSONResponse{
		NotImplementedJSONResponse: api.NotImplementedJSONResponse(errorModel(ctx, "USER_SERVICE_NOT_IMPLEMENTED", "user business service is not connected", nil)),
	}, nil
}

// ListCategories returns all user-facing item categories.
func (h *Handler) ListCategories(ctx context.Context, _ api.ListCategoriesRequestObject) (api.ListCategoriesResponseObject, error) {
	categories, err := h.categories.List(ctx)
	if err != nil {
		return api.ListCategories500JSONResponse{
			InternalErrorJSONResponse: api.InternalErrorJSONResponse(errorModel(ctx, "INTERNAL_ERROR", "failed to list categories", nil)),
		}, nil
	}
	result := make([]api.Category, 0, len(categories))
	for _, c := range categories {
		result = append(result, api.Category{
			Id:       c.ID,
			Name:     c.Name,
			IsSystem: c.IsSystem,
		})
	}
	return api.ListCategories200JSONResponse(api.CategoryList{Categories: result}), nil
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

	if err := h.categories.ValidateCategory(ctx, request.Body.CategoryId); err != nil {
		return api.CreateItem422JSONResponse{
			ValidationErrorJSONResponse: api.ValidationErrorJSONResponse(errorModel(ctx, "VALIDATION_ERROR", "category validation failed", map[string]any{
				"categoryId": err.Error(),
			})),
		}, nil
	}
	offerCategoryID := request.Body.CategoryId

	item, err := h.items.Create(ctx, current.UserID, items.CreateInput{
		OfferTitle:       request.Body.OfferTitle,
		OfferDescription: request.Body.OfferDescription,
		Wishes:           request.Body.Wishes,
		ImageURLs:        imageURLs,
		OfferCategoryID:  &offerCategoryID,
		Condition:        string(request.Body.Condition),
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

// UpdateItem changes an exchange item's descriptions or withdraws it from
// matching while retaining the item in completed exchange history.
func (h *Handler) UpdateItem(ctx context.Context, request api.UpdateItemRequestObject) (api.UpdateItemResponseObject, error) {
	current, ok := session.Current(ctx)
	if !ok {
		return api.UpdateItem401JSONResponse{
			UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(sessionRequired(ctx)),
		}, nil
	}
	if request.Body == nil {
		return api.UpdateItem422JSONResponse{
			ValidationErrorJSONResponse: api.ValidationErrorJSONResponse(errorModel(ctx, "VALIDATION_ERROR", "item update is required", map[string]any{"request": "JSON request body is required"})),
		}, nil
	}

	withdraw := request.Body.Withdraw != nil && *request.Body.Withdraw

	var offerCategoryID *int32
	if request.Body.CategoryId != nil {
		if err := h.categories.ValidateCategory(ctx, *request.Body.CategoryId); err != nil {
			return api.UpdateItem422JSONResponse{
				ValidationErrorJSONResponse: api.ValidationErrorJSONResponse(errorModel(ctx, "VALIDATION_ERROR", "category validation failed", map[string]any{
					"categoryId": err.Error(),
				})),
			}, nil
		}
		offerCategoryID = request.Body.CategoryId
	}

	var condition *string
	if request.Body.Condition != nil {
		value := string(*request.Body.Condition)
		condition = &value
	}
	item, err := h.items.Update(ctx, current.UserID, request.ItemId, items.UpdateInput{
		OfferTitle:       request.Body.OfferTitle,
		OfferDescription: request.Body.OfferDescription,
		Wishes: func() []string {
			if request.Body.Wishes != nil {
				return *request.Body.Wishes
			} else {
				return nil
			}
		}(),
		OfferCategoryID: offerCategoryID,
		Condition:       condition,
		Withdraw:        withdraw,
	})
	if err != nil {
		var validationError *items.ValidationError
		switch {
		case errors.As(err, &validationError):
			details := make(map[string]any, len(validationError.Fields))
			for field, message := range validationError.Fields {
				details[field] = message
			}
			return api.UpdateItem422JSONResponse{
				ValidationErrorJSONResponse: api.ValidationErrorJSONResponse(errorModel(ctx, "VALIDATION_ERROR", "item validation failed", details)),
			}, nil
		case errors.Is(err, items.ErrNotFound):
			return api.UpdateItem404JSONResponse{
				NotFoundJSONResponse: api.NotFoundJSONResponse(errorModel(ctx, "ITEM_NOT_FOUND", "item not found", nil)),
			}, nil
		case errors.Is(err, items.ErrForbidden):
			return api.UpdateItem403JSONResponse{
				ForbiddenJSONResponse: api.ForbiddenJSONResponse(errorModel(ctx, "ITEM_FORBIDDEN", "item does not belong to the current user", nil)),
			}, nil
		case errors.Is(err, items.ErrConflict):
			return api.UpdateItem409JSONResponse{
				ConflictJSONResponse: api.ConflictJSONResponse(errorModel(ctx, "ITEM_LOCKED", "a locked item cannot be changed", nil)),
			}, nil
		default:
			return api.UpdateItem500JSONResponse{
				InternalErrorJSONResponse: api.InternalErrorJSONResponse(errorModel(ctx, "INTERNAL_ERROR", "failed to update item", nil)),
			}, nil
		}
	}
	return api.UpdateItem200JSONResponse(itemModel(item)), nil
}

// ResolveItemCategories применяет предложенный ручной выбор владельца карточки.
func (h *Handler) ResolveItemCategories(
	ctx context.Context,
	request api.ResolveItemCategoriesRequestObject,
) (api.ResolveItemCategoriesResponseObject, error) {
	current, ok := session.Current(ctx)
	if !ok {
		return api.ResolveItemCategories401JSONResponse{
			UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(sessionRequired(ctx)),
		}, nil
	}
	if request.Body == nil {
		return api.ResolveItemCategories400JSONResponse{
			BadRequestJSONResponse: api.BadRequestJSONResponse(errorModel(ctx, "INVALID_REQUEST", "JSON request body is required", nil)),
		}, nil
	}

	input := items.CategoryDecisionInput{OfferCategoryID: request.Body.OfferCategoryId}
	if request.Body.OfferCategoryId != nil {
		if err := h.categories.ValidateCategory(ctx, *request.Body.OfferCategoryId); err != nil {
			return api.ResolveItemCategories422JSONResponse{
				ValidationErrorJSONResponse: api.ValidationErrorJSONResponse(errorModel(ctx, "VALIDATION_ERROR", "category validation failed", map[string]any{
					"offerCategoryId": err.Error(),
				})),
			}, nil
		}
	}
	if request.Body.Wishes != nil {
		input.Wishes = make([]items.WishCategoryDecision, 0, len(*request.Body.Wishes))
		for _, decision := range *request.Body.Wishes {
			if err := h.categories.ValidateCategory(ctx, decision.CategoryId); err != nil {
				return api.ResolveItemCategories422JSONResponse{
					ValidationErrorJSONResponse: api.ValidationErrorJSONResponse(errorModel(ctx, "VALIDATION_ERROR", "category validation failed", map[string]any{
						"wishes": err.Error(),
					})),
				}, nil
			}
			input.Wishes = append(input.Wishes, items.WishCategoryDecision{
				WishID:     decision.WishId,
				CategoryID: decision.CategoryId,
			})
		}
	}

	item, err := h.items.ResolveCategories(ctx, current.UserID, request.ItemId, input)
	if err != nil {
		var validationError *items.ValidationError
		switch {
		case errors.As(err, &validationError):
			details := make(map[string]any, len(validationError.Fields))
			for field, message := range validationError.Fields {
				details[field] = message
			}
			return api.ResolveItemCategories422JSONResponse{
				ValidationErrorJSONResponse: api.ValidationErrorJSONResponse(errorModel(ctx, "VALIDATION_ERROR", "category decision is invalid", details)),
			}, nil
		case errors.Is(err, items.ErrNotFound):
			return api.ResolveItemCategories404JSONResponse{
				NotFoundJSONResponse: api.NotFoundJSONResponse(errorModel(ctx, "ITEM_NOT_FOUND", "item not found", nil)),
			}, nil
		case errors.Is(err, items.ErrForbidden):
			return api.ResolveItemCategories403JSONResponse{
				ForbiddenJSONResponse: api.ForbiddenJSONResponse(errorModel(ctx, "ITEM_FORBIDDEN", "item does not belong to the current user", nil)),
			}, nil
		case errors.Is(err, items.ErrConflict):
			return api.ResolveItemCategories409JSONResponse{
				ConflictJSONResponse: api.ConflictJSONResponse(errorModel(ctx, "CATEGORY_DECISION_NOT_REQUIRED", "item is not waiting for category input", nil)),
			}, nil
		default:
			h.logger.Error("resolve item categories", zap.Int64("item_id", request.ItemId), zap.Error(err))
			return api.ResolveItemCategories500JSONResponse{
				InternalErrorJSONResponse: api.InternalErrorJSONResponse(errorModel(ctx, "INTERNAL_ERROR", "failed to resolve item categories", nil)),
			}, nil
		}
	}

	return api.ResolveItemCategories200JSONResponse(itemModel(item)), nil
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

// ConfirmChainReceipt confirms only the incoming item selected from the
// authenticated participant and may atomically complete the whole chain.
func (h *Handler) ConfirmChainReceipt(ctx context.Context, request api.ConfirmChainReceiptRequestObject) (api.ConfirmChainReceiptResponseObject, error) {
	current, ok := session.Current(ctx)
	if !ok {
		return api.ConfirmChainReceipt401JSONResponse{
			UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(sessionRequired(ctx)),
		}, nil
	}

	receipt, err := h.admin.ConfirmReceipt(ctx, current.UserID, request.ChainId)
	if err != nil {
		mapped := adminErrorModel(ctx, err)
		switch {
		case isAdminValidationError(err):
			return api.ConfirmChainReceipt400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse(mapped)}, nil
		case errors.Is(err, adminmodel.ErrReceiptForbidden):
			return api.ConfirmChainReceipt403JSONResponse{ForbiddenJSONResponse: api.ForbiddenJSONResponse(mapped)}, nil
		case errors.Is(err, adminmodel.ErrChainNotFound):
			return api.ConfirmChainReceipt404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse(mapped)}, nil
		case errors.Is(err, adminmodel.ErrTransitionConflict):
			return api.ConfirmChainReceipt409JSONResponse{ConflictJSONResponse: api.ConflictJSONResponse(mapped)}, nil
		default:
			h.logger.Error("confirm chain receipt", zap.Int64("actor_id", current.UserID), zap.Int64("chain_id", request.ChainId), zap.Error(err))
			return api.ConfirmChainReceipt500JSONResponse{InternalErrorJSONResponse: api.InternalErrorJSONResponse(mapped)}, nil
		}
	}
	return api.ConfirmChainReceipt200JSONResponse{
		Delivery:    adminDeliveryModel(receipt.Delivery),
		ChainStatus: api.ChainStatus(receipt.ChainStatus),
	}, nil
}

// ListChatMessages возвращает историю диалога или ожидает новое сообщение.
func (h *Handler) ListChatMessages(ctx context.Context, request api.ListChatMessagesRequestObject) (api.ListChatMessagesResponseObject, error) {
	current, ok := session.Current(ctx)
	if !ok {
		return api.ListChatMessages401JSONResponse{
			UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(sessionRequired(ctx)),
		}, nil
	}

	afterID := int64(0)
	if request.Params.AfterId != nil {
		afterID = *request.Params.AfterId
	}
	limit := 50
	if request.Params.Limit != nil {
		limit = *request.Params.Limit
	}
	waitSeconds := 0
	if request.Params.WaitSeconds != nil {
		waitSeconds = *request.Params.WaitSeconds
	}

	messages, err := h.chat.List(ctx, request.ItemId, current.UserID, request.CounterpartId, afterID, limit, time.Duration(waitSeconds)*time.Second)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		mapped := chatErrorModel(ctx, err)
		switch {
		case isChatValidationError(err):
			return api.ListChatMessages400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse(mapped)}, nil
		case errors.Is(err, chatmodel.ErrForbidden):
			return api.ListChatMessages403JSONResponse{ForbiddenJSONResponse: api.ForbiddenJSONResponse(mapped)}, nil
		case errors.Is(err, chatmodel.ErrItemNotFound), errors.Is(err, chatmodel.ErrThreadNotFound):
			return api.ListChatMessages404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse(mapped)}, nil
		default:
			h.logger.Error("list chat messages", zap.Int64("actor_id", current.UserID), zap.Int64("item_id", request.ItemId), zap.Int64("counterpart_id", request.CounterpartId), zap.Error(err))
			return api.ListChatMessages500JSONResponse{InternalErrorJSONResponse: api.InternalErrorJSONResponse(mapped)}, nil
		}
	}

	response := api.ChatMessageList{Messages: make([]api.ChatMessage, 0, len(messages))}
	for _, message := range messages {
		response.Messages = append(response.Messages, chatMessageModel(message))
	}
	if len(messages) > 0 {
		next := messages[len(messages)-1].ID
		response.NextAfterId = &next
	}
	return api.ListChatMessages200JSONResponse(response), nil
}

// SendChatMessage создаёт сообщение от имени пользователя текущей сессии.
func (h *Handler) SendChatMessage(ctx context.Context, request api.SendChatMessageRequestObject) (api.SendChatMessageResponseObject, error) {
	current, ok := session.Current(ctx)
	if !ok {
		return api.SendChatMessage401JSONResponse{
			UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(sessionRequired(ctx)),
		}, nil
	}
	if request.Body == nil {
		return api.SendChatMessage400JSONResponse{
			ValidationErrorJSONResponse: api.ValidationErrorJSONResponse(errorModel(ctx, "INVALID_REQUEST", "JSON request body is required", nil)),
		}, nil
	}

	message, created, err := h.chat.Send(ctx, request.ItemId, current.UserID, request.CounterpartId, request.Body.ClientMessageId, request.Body.Text)
	if err != nil {
		mapped := chatErrorModel(ctx, err)
		switch {
		case isChatValidationError(err):
			return api.SendChatMessage400JSONResponse{ValidationErrorJSONResponse: api.ValidationErrorJSONResponse(mapped)}, nil
		case errors.Is(err, chatmodel.ErrForbidden):
			return api.SendChatMessage403JSONResponse{ForbiddenJSONResponse: api.ForbiddenJSONResponse(mapped)}, nil
		case errors.Is(err, chatmodel.ErrItemNotFound), errors.Is(err, chatmodel.ErrThreadNotFound):
			return api.SendChatMessage404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse(mapped)}, nil
		case errors.Is(err, chatmodel.ErrIdempotencyConflict):
			return api.SendChatMessage409JSONResponse{ConflictJSONResponse: api.ConflictJSONResponse(mapped)}, nil
		default:
			h.logger.Error("send chat message", zap.Int64("actor_id", current.UserID), zap.Int64("item_id", request.ItemId), zap.Int64("counterpart_id", request.CounterpartId), zap.Error(err))
			return api.SendChatMessage500JSONResponse{InternalErrorJSONResponse: api.InternalErrorJSONResponse(mapped)}, nil
		}
	}

	response := chatMessageModel(message)
	if !created {
		return api.SendChatMessage200JSONResponse(response), nil
	}
	return api.SendChatMessage201JSONResponse(response), nil
}

// ListChatThreads возвращает все диалоги пользователя и общий счётчик непрочитанных сообщений.
func (h *Handler) ListChatThreads(ctx context.Context, _ api.ListChatThreadsRequestObject) (api.ListChatThreadsResponseObject, error) {
	current, ok := session.Current(ctx)
	if !ok {
		return api.ListChatThreads401JSONResponse{
			UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(sessionRequired(ctx)),
		}, nil
	}

	threads, err := h.chat.ListThreads(ctx, current.UserID)
	if err != nil {
		h.logger.Error("list chat threads", zap.Int64("actor_id", current.UserID), zap.Error(err))
		return api.ListChatThreads500JSONResponse{
			InternalErrorJSONResponse: api.InternalErrorJSONResponse(chatErrorModel(ctx, err)),
		}, nil
	}

	response := api.ChatThreadList{Threads: make([]api.ChatThread, 0, len(threads))}
	for _, thread := range threads {
		response.Threads = append(response.Threads, chatThreadModel(thread))
		response.TotalUnreadCount += thread.UnreadCount
	}
	return api.ListChatThreads200JSONResponse(response), nil
}

// MarkChatThreadRead продвигает отметку прочтения, не позволяя ей откатиться назад.
func (h *Handler) MarkChatThreadRead(ctx context.Context, request api.MarkChatThreadReadRequestObject) (api.MarkChatThreadReadResponseObject, error) {
	current, ok := session.Current(ctx)
	if !ok {
		return api.MarkChatThreadRead401JSONResponse{
			UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(sessionRequired(ctx)),
		}, nil
	}
	if request.Body == nil {
		return api.MarkChatThreadRead400JSONResponse{
			ValidationErrorJSONResponse: api.ValidationErrorJSONResponse(errorModel(ctx, "INVALID_REQUEST", "JSON request body is required", nil)),
		}, nil
	}

	readState, err := h.chat.MarkRead(ctx, request.ItemId, current.UserID, request.CounterpartId, request.Body.LastReadMessageId)
	if err != nil {
		mapped := chatErrorModel(ctx, err)
		switch {
		case isChatValidationError(err):
			return api.MarkChatThreadRead400JSONResponse{ValidationErrorJSONResponse: api.ValidationErrorJSONResponse(mapped)}, nil
		case errors.Is(err, chatmodel.ErrForbidden):
			return api.MarkChatThreadRead403JSONResponse{ForbiddenJSONResponse: api.ForbiddenJSONResponse(mapped)}, nil
		case errors.Is(err, chatmodel.ErrItemNotFound), errors.Is(err, chatmodel.ErrThreadNotFound), errors.Is(err, chatmodel.ErrMessageNotFound):
			return api.MarkChatThreadRead404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse(mapped)}, nil
		default:
			h.logger.Error("mark chat thread read", zap.Int64("actor_id", current.UserID), zap.Int64("item_id", request.ItemId), zap.Int64("counterpart_id", request.CounterpartId), zap.Error(err))
			return api.MarkChatThreadRead500JSONResponse{InternalErrorJSONResponse: api.InternalErrorJSONResponse(mapped)}, nil
		}
	}

	return api.MarkChatThreadRead200JSONResponse(chatReadStateModel(readState)), nil
}

// ListAdminDeliveries returns assembled-chain item hand-offs to an authenticated pickup-point administrator.
func (h *Handler) ListAdminChains(ctx context.Context, request api.ListAdminChainsRequestObject) (api.ListAdminChainsResponseObject, error) {
	current, ok := session.Current(ctx)
	if !ok {
		return api.ListAdminChains401JSONResponse{UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(sessionRequired(ctx))}, nil
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
			return api.ListAdminChains400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse(errorModel(ctx, "INVALID_CURSOR", "cursor must be a non-negative integer", nil))}, nil
		}
		afterID = parsed
	}
	chains, next, err := h.admin.ListChains(ctx, current.UserID, status, afterID, limit)
	if err != nil {
		mapped := adminErrorModel(ctx, err)
		switch {
		case isAdminValidationError(err):
			return api.ListAdminChains400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse(mapped)}, nil
		case errors.Is(err, adminmodel.ErrForbidden):
			return api.ListAdminChains403JSONResponse{ForbiddenJSONResponse: api.ForbiddenJSONResponse(mapped)}, nil
		default:
			h.logger.Error("list admin chains", zap.Int64("actor_id", current.UserID), zap.Error(err))
			return api.ListAdminChains500JSONResponse{InternalErrorJSONResponse: api.InternalErrorJSONResponse(mapped)}, nil
		}
	}
	response := api.AdminChainList{Chains: make([]api.AdminChainSummary, 0, len(chains))}
	for _, chain := range chains {
		response.Chains = append(response.Chains, adminChainSummaryModel(chain))
	}
	if next != nil {
		cursor := strconv.FormatInt(*next, 10)
		response.NextCursor = &cursor
	}
	return api.ListAdminChains200JSONResponse(response), nil
}

func (h *Handler) GetAdminChain(ctx context.Context, request api.GetAdminChainRequestObject) (api.GetAdminChainResponseObject, error) {
	current, ok := session.Current(ctx)
	if !ok {
		return api.GetAdminChain401JSONResponse{UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(sessionRequired(ctx))}, nil
	}
	chain, err := h.admin.GetChain(ctx, current.UserID, request.ChainId)
	if err != nil {
		mapped := adminErrorModel(ctx, err)
		switch {
		case errors.Is(err, adminmodel.ErrForbidden):
			return api.GetAdminChain403JSONResponse{ForbiddenJSONResponse: api.ForbiddenJSONResponse(mapped)}, nil
		case errors.Is(err, adminmodel.ErrChainNotFound):
			return api.GetAdminChain404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse(mapped)}, nil
		default:
			h.logger.Error("get admin chain", zap.Int64("actor_id", current.UserID), zap.Int64("chain_id", request.ChainId), zap.Error(err))
			return api.GetAdminChain500JSONResponse{InternalErrorJSONResponse: api.InternalErrorJSONResponse(mapped)}, nil
		}
	}
	return api.GetAdminChain200JSONResponse(adminChainModel(chain)), nil
}

func (h *Handler) ConfirmAdminParticipantReceipt(ctx context.Context, request api.ConfirmAdminParticipantReceiptRequestObject) (api.ConfirmAdminParticipantReceiptResponseObject, error) {
	current, ok := session.Current(ctx)
	if !ok {
		return api.ConfirmAdminParticipantReceipt401JSONResponse{UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(sessionRequired(ctx))}, nil
	}
	receipt, err := h.admin.ConfirmParticipantReceipt(ctx, current.UserID, request.ChainId, request.ParticipantId)
	if err != nil {
		mapped := adminErrorModel(ctx, err)
		switch {
		case isAdminValidationError(err):
			return api.ConfirmAdminParticipantReceipt400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse(mapped)}, nil
		case errors.Is(err, adminmodel.ErrForbidden):
			return api.ConfirmAdminParticipantReceipt403JSONResponse{ForbiddenJSONResponse: api.ForbiddenJSONResponse(mapped)}, nil
		case errors.Is(err, adminmodel.ErrChainNotFound), errors.Is(err, adminmodel.ErrDeliveryNotFound):
			return api.ConfirmAdminParticipantReceipt404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse(mapped)}, nil
		case errors.Is(err, adminmodel.ErrTransitionConflict):
			return api.ConfirmAdminParticipantReceipt409JSONResponse{ConflictJSONResponse: api.ConflictJSONResponse(mapped)}, nil
		default:
			h.logger.Error("confirm participant receipt", zap.Int64("actor_id", current.UserID), zap.Int64("chain_id", request.ChainId), zap.Int64("participant_id", request.ParticipantId), zap.Error(err))
			return api.ConfirmAdminParticipantReceipt500JSONResponse{InternalErrorJSONResponse: api.InternalErrorJSONResponse(mapped)}, nil
		}
	}
	return api.ConfirmAdminParticipantReceipt200JSONResponse(api.ChainReceipt{Delivery: adminDeliveryModel(receipt.Delivery), ChainStatus: api.ChainStatus(receipt.ChainStatus)}), nil
}

// ListAdminDeliveries returns assembled-chain item hand-offs to an authenticated pickup-point administrator.
func (h *Handler) ListAdminDeliveries(ctx context.Context, request api.ListAdminDeliveriesRequestObject) (api.ListAdminDeliveriesResponseObject, error) {
	current, ok := session.Current(ctx)
	if !ok {
		return api.ListAdminDeliveries401JSONResponse{
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
			return api.ListAdminDeliveries400JSONResponse{
				BadRequestJSONResponse: api.BadRequestJSONResponse(errorModel(ctx, "INVALID_CURSOR", "cursor must be a non-negative integer", nil)),
			}, nil
		}
		afterID = parsed
	}

	deliveries, next, err := h.admin.ListDeliveries(ctx, current.UserID, status, afterID, limit)
	if err != nil {
		switch {
		case isAdminValidationError(err):
			return api.ListAdminDeliveries400JSONResponse{
				BadRequestJSONResponse: api.BadRequestJSONResponse(adminErrorModel(ctx, err)),
			}, nil
		case errors.Is(err, adminmodel.ErrForbidden):
			return api.ListAdminDeliveries403JSONResponse{
				ForbiddenJSONResponse: api.ForbiddenJSONResponse(adminErrorModel(ctx, err)),
			}, nil
		default:
			h.logger.Error("list admin deliveries", zap.Int64("actor_id", current.UserID), zap.Error(err))
			return api.ListAdminDeliveries500JSONResponse{
				InternalErrorJSONResponse: api.InternalErrorJSONResponse(adminErrorModel(ctx, err)),
			}, nil
		}
	}

	response := api.AdminDeliveryList{Deliveries: make([]api.AdminDelivery, 0, len(deliveries))}
	for _, delivery := range deliveries {
		response.Deliveries = append(response.Deliveries, adminDeliveryModel(delivery))
	}
	if next != nil {
		cursor := strconv.FormatInt(*next, 10)
		response.NextCursor = &cursor
	}
	return api.ListAdminDeliveries200JSONResponse(response), nil
}

// TransitionAdminDelivery confirms pickup-point receipt, dispatch, or recipient hand-off.
func (h *Handler) TransitionAdminDelivery(ctx context.Context, request api.TransitionAdminDeliveryRequestObject) (api.TransitionAdminDeliveryResponseObject, error) {
	current, ok := session.Current(ctx)
	if !ok {
		return api.TransitionAdminDelivery401JSONResponse{
			UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(sessionRequired(ctx)),
		}, nil
	}
	if request.Body == nil {
		return api.TransitionAdminDelivery400JSONResponse{
			ValidationErrorJSONResponse: api.ValidationErrorJSONResponse(errorModel(ctx, "INVALID_REQUEST", "JSON request body is required", nil)),
		}, nil
	}

	delivery, err := h.admin.TransitionDelivery(ctx, current.UserID, request.DeliveryId, string(request.Body.Status))
	if err != nil {
		mapped := adminErrorModel(ctx, err)
		switch {
		case isAdminValidationError(err):
			return api.TransitionAdminDelivery400JSONResponse{ValidationErrorJSONResponse: api.ValidationErrorJSONResponse(mapped)}, nil
		case errors.Is(err, adminmodel.ErrForbidden):
			return api.TransitionAdminDelivery403JSONResponse{ForbiddenJSONResponse: api.ForbiddenJSONResponse(mapped)}, nil
		case errors.Is(err, adminmodel.ErrDeliveryNotFound):
			return api.TransitionAdminDelivery404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse(mapped)}, nil
		case errors.Is(err, adminmodel.ErrTransitionConflict):
			return api.TransitionAdminDelivery409JSONResponse{ConflictJSONResponse: api.ConflictJSONResponse(mapped)}, nil
		default:
			h.logger.Error("transition admin delivery", zap.Int64("actor_id", current.UserID), zap.Int64("delivery_id", request.DeliveryId), zap.Error(err))
			return api.TransitionAdminDelivery500JSONResponse{InternalErrorJSONResponse: api.InternalErrorJSONResponse(mapped)}, nil
		}
	}
	return api.TransitionAdminDelivery200JSONResponse(adminDeliveryModel(delivery)), nil
}

// ListNotifications returns cursor-paginated notifications for the current user.
func (h *Handler) ListNotifications(ctx context.Context, request api.ListNotificationsRequestObject) (api.ListNotificationsResponseObject, error) {
	current, ok := session.Current(ctx)
	if !ok {
		return api.ListNotifications401JSONResponse{
			UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(sessionRequired(ctx)),
		}, nil
	}

	limit := 20
	if request.Params.Limit != nil {
		limit = *request.Params.Limit
	}

	cursor := int64(0)
	if request.Params.Cursor != nil {
		parsed, err := strconv.ParseInt(*request.Params.Cursor, 10, 64)
		if err != nil || parsed < 0 {
			return api.ListNotifications400JSONResponse{
				BadRequestJSONResponse: api.BadRequestJSONResponse(errorModel(ctx, "INVALID_CURSOR", "cursor must be a non-negative integer", nil)),
			}, nil
		}
		cursor = parsed
	}

	result, err := h.notifications.List(ctx, current.UserID, cursor, limit)
	if err != nil {
		var validationError *notificationmodel.ValidationError
		if errors.As(err, &validationError) {
			return api.ListNotifications400JSONResponse{
				BadRequestJSONResponse: api.BadRequestJSONResponse(errorModel(ctx, "VALIDATION_ERROR", err.Error(), nil)),
			}, nil
		}
		h.logger.Error("list notifications", zap.Int64("user_id", current.UserID), zap.Error(err))
		return api.ListNotifications500JSONResponse{
			InternalErrorJSONResponse: api.InternalErrorJSONResponse(errorModel(ctx, "INTERNAL_ERROR", "failed to list notifications", nil)),
		}, nil
	}

	response := api.NotificationList{
		Notifications: make([]api.AppNotification, 0, len(result.Notifications)),
		TotalUnread:   result.TotalUnread,
	}
	for _, n := range result.Notifications {
		response.Notifications = append(response.Notifications, notificationModel(n))
	}
	if result.NextCursor != nil {
		c := strconv.FormatInt(*result.NextCursor, 10)
		response.NextCursor = &c
	}

	return api.ListNotifications200JSONResponse(response), nil
}

// MarkNotificationsRead marks selected or all notifications as read.
func (h *Handler) MarkNotificationsRead(ctx context.Context, request api.MarkNotificationsReadRequestObject) (api.MarkNotificationsReadResponseObject, error) {
	current, ok := session.Current(ctx)
	if !ok {
		return api.MarkNotificationsRead401JSONResponse{
			UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(sessionRequired(ctx)),
		}, nil
	}

	var ids []int64
	if request.Body != nil && request.Body.Ids != nil {
		ids = *request.Body.Ids
	}

	if err := h.notifications.MarkRead(ctx, current.UserID, ids); err != nil {
		var validationError *notificationmodel.ValidationError
		if errors.As(err, &validationError) {
			return api.MarkNotificationsRead400JSONResponse{
				BadRequestJSONResponse: api.BadRequestJSONResponse(errorModel(ctx, "VALIDATION_ERROR", err.Error(), nil)),
			}, nil
		}
		h.logger.Error("mark notifications read", zap.Int64("user_id", current.UserID), zap.Error(err))
		return api.MarkNotificationsRead500JSONResponse{
			InternalErrorJSONResponse: api.InternalErrorJSONResponse(errorModel(ctx, "INTERNAL_ERROR", "failed to mark notifications as read", nil)),
		}, nil
	}

	result, err := h.notifications.List(ctx, current.UserID, 0, 20)
	if err != nil {
		h.logger.Error("list notifications after mark read", zap.Int64("user_id", current.UserID), zap.Error(err))
		return api.MarkNotificationsRead500JSONResponse{
			InternalErrorJSONResponse: api.InternalErrorJSONResponse(errorModel(ctx, "INTERNAL_ERROR", "failed to list notifications", nil)),
		}, nil
	}

	response := api.NotificationList{
		Notifications: make([]api.AppNotification, 0, len(result.Notifications)),
		TotalUnread:   result.TotalUnread,
	}
	for _, n := range result.Notifications {
		response.Notifications = append(response.Notifications, notificationModel(n))
	}
	if result.NextCursor != nil {
		c := strconv.FormatInt(*result.NextCursor, 10)
		response.NextCursor = &c
	}

	return api.MarkNotificationsRead200JSONResponse(response), nil
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

// ListBlocks returns the current user's personal blacklist.
func (h *Handler) ListBlocks(ctx context.Context, request api.ListBlocksRequestObject) (api.ListBlocksResponseObject, error) {
	current, ok := session.Current(ctx)
	if !ok {
		return api.ListBlocks401JSONResponse{
			UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(sessionRequired(ctx)),
		}, nil
	}

	limit := 20
	if request.Params.Limit != nil {
		limit = *request.Params.Limit
	}
	afterID := int64(0)
	if request.Params.Cursor != nil {
		parsed, err := strconv.ParseInt(*request.Params.Cursor, 10, 64)
		if err != nil || parsed < 0 {
			return api.ListBlocks400JSONResponse{
				BadRequestJSONResponse: api.BadRequestJSONResponse(errorModel(ctx, "INVALID_CURSOR", "cursor must be a non-negative integer", nil)),
			}, nil
		}
		afterID = parsed
	}

	blocks, next, err := h.blocklist.List(ctx, current.UserID, afterID, limit)
	if err != nil {
		mapped := blocklistErrorModel(ctx, err)
		if isBlocklistValidationError(err) {
			return api.ListBlocks400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse(mapped)}, nil
		}
		h.logger.Error("list blocks", zap.Int64("actor_id", current.UserID), zap.Error(err))
		return api.ListBlocks500JSONResponse{
			InternalErrorJSONResponse: api.InternalErrorJSONResponse(mapped),
		}, nil
	}

	response := api.BlockList{Blocks: make([]api.Block, 0, len(blocks))}
	for _, block := range blocks {
		response.Blocks = append(response.Blocks, blockModel(block))
	}
	if next != nil {
		cursor := strconv.FormatInt(*next, 10)
		response.NextCursor = &cursor
	}
	return api.ListBlocks200JSONResponse(response), nil
}

// BlockUser blocks another user and cancels shared non-terminal chains.
func (h *Handler) BlockUser(ctx context.Context, request api.BlockUserRequestObject) (api.BlockUserResponseObject, error) {
	current, ok := session.Current(ctx)
	if !ok {
		return api.BlockUser401JSONResponse{
			UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(sessionRequired(ctx)),
		}, nil
	}
	if request.Body == nil {
		return api.BlockUser400JSONResponse{
			BadRequestJSONResponse: api.BadRequestJSONResponse(errorModel(ctx, "INVALID_REQUEST", "JSON request body is required", nil)),
		}, nil
	}

	block, err := h.blocklist.Block(ctx, current.UserID, request.Body.BlockedUserId)
	if err != nil {
		mapped := blocklistErrorModel(ctx, err)
		switch {
		case isBlocklistValidationError(err), errors.Is(err, blocklistmodel.ErrSelfBlock):
			return api.BlockUser400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse(mapped)}, nil
		case errors.Is(err, blocklistmodel.ErrTargetNotFound):
			return api.BlockUser404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse(mapped)}, nil
		default:
			h.logger.Error("block user", zap.Int64("actor_id", current.UserID), zap.Int64("blocked_id", request.Body.BlockedUserId), zap.Error(err))
			return api.BlockUser500JSONResponse{InternalErrorJSONResponse: api.InternalErrorJSONResponse(mapped)}, nil
		}
	}
	return api.BlockUser200JSONResponse(blockModel(block)), nil
}

// UnblockUser removes a user from the personal blacklist.
func (h *Handler) UnblockUser(ctx context.Context, request api.UnblockUserRequestObject) (api.UnblockUserResponseObject, error) {
	current, ok := session.Current(ctx)
	if !ok {
		return api.UnblockUser401JSONResponse{
			UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(sessionRequired(ctx)),
		}, nil
	}

	if err := h.blocklist.Unblock(ctx, current.UserID, request.BlockedUserId); err != nil {
		mapped := blocklistErrorModel(ctx, err)
		switch {
		case isBlocklistValidationError(err):
			return api.UnblockUser400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse(mapped)}, nil
		case errors.Is(err, blocklistmodel.ErrTargetNotFound):
			return api.UnblockUser404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse(mapped)}, nil
		}
		h.logger.Error("unblock user", zap.Int64("actor_id", current.UserID), zap.Int64("blocked_id", request.BlockedUserId), zap.Error(err))
		return api.UnblockUser500JSONResponse{InternalErrorJSONResponse: api.InternalErrorJSONResponse(mapped)}, nil
	}
	return api.UnblockUser204Response{}, nil
}

// CreateReport creates or returns an idempotent message report.
func (h *Handler) CreateReport(ctx context.Context, request api.CreateReportRequestObject) (api.CreateReportResponseObject, error) {
	current, ok := session.Current(ctx)
	if !ok {
		return api.CreateReport401JSONResponse{
			UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(sessionRequired(ctx)),
		}, nil
	}
	if request.Body == nil {
		return api.CreateReport400JSONResponse{
			BadRequestJSONResponse: api.BadRequestJSONResponse(errorModel(ctx, "INVALID_REQUEST", "JSON request body is required", nil)),
		}, nil
	}

	var comment string
	if request.Body.Comment != nil {
		comment = *request.Body.Comment
	}
	report, created, err := h.moderation.CreateReport(ctx, current.UserID, request.Body.MessageId, string(request.Body.Reason), comment)
	if err != nil {
		mapped := moderationErrorModel(ctx, err)
		switch {
		case isModerationValidationError(err), errors.Is(err, moderationmodel.ErrSelfReport):
			return api.CreateReport400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse(mapped)}, nil
		case errors.Is(err, moderationmodel.ErrNotFound), errors.Is(err, moderationmodel.ErrReportUnavailable):
			return api.CreateReport404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse(mapped)}, nil
		default:
			h.logger.Error("create report", zap.Int64("reporter_id", current.UserID), zap.Int64("message_id", request.Body.MessageId), zap.Error(err))
			return api.CreateReport500JSONResponse{InternalErrorJSONResponse: api.InternalErrorJSONResponse(mapped)}, nil
		}
	}

	response := reportModel(report)
	if !created {
		return api.CreateReport200JSONResponse(response), nil
	}
	return api.CreateReport201JSONResponse(response), nil
}

// CreateUserReport creates or returns an idempotent complaint about another user.
func (h *Handler) CreateUserReport(ctx context.Context, request api.CreateUserReportRequestObject) (api.CreateUserReportResponseObject, error) {
	current, ok := session.Current(ctx)
	if !ok {
		return api.CreateUserReport401JSONResponse{
			UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(sessionRequired(ctx)),
		}, nil
	}
	if request.Body == nil {
		return api.CreateUserReport400JSONResponse{
			BadRequestJSONResponse: api.BadRequestJSONResponse(errorModel(ctx, "INVALID_REQUEST", "JSON request body is required", nil)),
		}, nil
	}

	var comment string
	if request.Body.Comment != nil {
		comment = *request.Body.Comment
	}
	report, created, err := h.moderation.CreateUserReport(ctx, current.UserID, request.UserId, request.Body.ChainId, string(request.Body.Reason), comment)
	if err != nil {
		mapped := moderationErrorModel(ctx, err)
		switch {
		case isModerationValidationError(err), errors.Is(err, moderationmodel.ErrSelfReport):
			return api.CreateUserReport400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse(mapped)}, nil
		case errors.Is(err, moderationmodel.ErrNotFound), errors.Is(err, moderationmodel.ErrUserReportUnavailable):
			return api.CreateUserReport404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse(mapped)}, nil
		default:
			h.logger.Error("create user report", zap.Int64("reporter_id", current.UserID), zap.Int64("target_user_id", request.UserId), zap.Error(err))
			return api.CreateUserReport500JSONResponse{InternalErrorJSONResponse: api.InternalErrorJSONResponse(mapped)}, nil
		}
	}

	response := userReportModel(report)
	if !created {
		return api.CreateUserReport200JSONResponse(response), nil
	}
	return api.CreateUserReport201JSONResponse(response), nil
}

// ListAdminReports returns the moderation queue.
func (h *Handler) ListAdminReports(ctx context.Context, request api.ListAdminReportsRequestObject) (api.ListAdminReportsResponseObject, error) {
	current, ok := session.Current(ctx)
	if !ok {
		return api.ListAdminReports401JSONResponse{
			UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(sessionRequired(ctx)),
		}, nil
	}

	limit := 20
	if request.Params.Limit != nil {
		limit = *request.Params.Limit
	}
	afterID := int64(0)
	if request.Params.Cursor != nil {
		parsed, err := strconv.ParseInt(*request.Params.Cursor, 10, 64)
		if err != nil || parsed < 0 {
			return api.ListAdminReports400JSONResponse{
				BadRequestJSONResponse: api.BadRequestJSONResponse(errorModel(ctx, "INVALID_CURSOR", "cursor must be a non-negative integer", nil)),
			}, nil
		}
		afterID = parsed
	}

	filter := moderationmodel.ReportFilter{}
	if request.Params.Status != nil {
		filter.Status = string(*request.Params.Status)
	}
	if request.Params.Reason != nil {
		filter.Reason = string(*request.Params.Reason)
	}
	if request.Params.AssigneeId != nil {
		filter.AssigneeID = *request.Params.AssigneeId
	}
	if request.Params.Unassigned != nil {
		filter.Unassigned = *request.Params.Unassigned
	}

	reports, next, err := h.moderation.ListReports(ctx, current.UserID, filter, afterID, limit)
	if err != nil {
		mapped := moderationErrorModel(ctx, err)
		switch {
		case isModerationValidationError(err):
			return api.ListAdminReports400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse(mapped)}, nil
		case errors.Is(err, moderationmodel.ErrForbidden):
			return api.ListAdminReports403JSONResponse{ForbiddenJSONResponse: api.ForbiddenJSONResponse(mapped)}, nil
		default:
			h.logger.Error("list admin reports", zap.Int64("actor_id", current.UserID), zap.Error(err))
			return api.ListAdminReports500JSONResponse{InternalErrorJSONResponse: api.InternalErrorJSONResponse(mapped)}, nil
		}
	}

	response := api.MessageReportList{Reports: make([]api.MessageReport, 0, len(reports))}
	for _, report := range reports {
		response.Reports = append(response.Reports, reportModel(report))
	}
	if next != nil {
		cursor := strconv.FormatInt(*next, 10)
		response.NextCursor = &cursor
	}
	return api.ListAdminReports200JSONResponse(response), nil
}

// GetAdminReport returns one report with the reported message and thread context.
func (h *Handler) GetAdminReport(ctx context.Context, request api.GetAdminReportRequestObject) (api.GetAdminReportResponseObject, error) {
	current, ok := session.Current(ctx)
	if !ok {
		return api.GetAdminReport401JSONResponse{
			UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(sessionRequired(ctx)),
		}, nil
	}

	detail, err := h.moderation.GetReport(ctx, current.UserID, request.ReportId)
	if err != nil {
		mapped := moderationErrorModel(ctx, err)
		switch {
		case isModerationValidationError(err):
			return api.GetAdminReport400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse(mapped)}, nil
		case errors.Is(err, moderationmodel.ErrForbidden):
			return api.GetAdminReport403JSONResponse{ForbiddenJSONResponse: api.ForbiddenJSONResponse(mapped)}, nil
		case errors.Is(err, moderationmodel.ErrNotFound):
			return api.GetAdminReport404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse(mapped)}, nil
		default:
			h.logger.Error("get admin report", zap.Int64("actor_id", current.UserID), zap.Int64("report_id", request.ReportId), zap.Error(err))
			return api.GetAdminReport500JSONResponse{InternalErrorJSONResponse: api.InternalErrorJSONResponse(mapped)}, nil
		}
	}
	return api.GetAdminReport200JSONResponse(reportDetailModel(detail)), nil
}

// AssignReport assigns an open report to the current administrator.
func (h *Handler) AssignReport(ctx context.Context, request api.AssignReportRequestObject) (api.AssignReportResponseObject, error) {
	current, ok := session.Current(ctx)
	if !ok {
		return api.AssignReport401JSONResponse{
			UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(sessionRequired(ctx)),
		}, nil
	}

	report, err := h.moderation.Assign(ctx, current.UserID, request.ReportId)
	if err != nil {
		mapped := moderationErrorModel(ctx, err)
		switch {
		case isModerationValidationError(err):
			return api.AssignReport400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse(mapped)}, nil
		case errors.Is(err, moderationmodel.ErrForbidden):
			return api.AssignReport403JSONResponse{ForbiddenJSONResponse: api.ForbiddenJSONResponse(mapped)}, nil
		case errors.Is(err, moderationmodel.ErrNotFound):
			return api.AssignReport404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse(mapped)}, nil
		case errors.Is(err, moderationmodel.ErrAlreadyAssigned), errors.Is(err, moderationmodel.ErrStateConflict):
			return api.AssignReport409JSONResponse{ConflictJSONResponse: api.ConflictJSONResponse(mapped)}, nil
		default:
			h.logger.Error("assign report", zap.Int64("actor_id", current.UserID), zap.Int64("report_id", request.ReportId), zap.Error(err))
			return api.AssignReport500JSONResponse{InternalErrorJSONResponse: api.InternalErrorJSONResponse(mapped)}, nil
		}
	}
	return api.AssignReport200JSONResponse(reportModel(report)), nil
}

// DecideReport resolves or rejects an assigned open report.
func (h *Handler) DecideReport(ctx context.Context, request api.DecideReportRequestObject) (api.DecideReportResponseObject, error) {
	current, ok := session.Current(ctx)
	if !ok {
		return api.DecideReport401JSONResponse{
			UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(sessionRequired(ctx)),
		}, nil
	}
	if request.Body == nil {
		return api.DecideReport400JSONResponse{
			BadRequestJSONResponse: api.BadRequestJSONResponse(errorModel(ctx, "INVALID_REQUEST", "JSON request body is required", nil)),
		}, nil
	}

	report, err := h.moderation.Decide(ctx, current.UserID, request.ReportId, string(request.Body.Decision), request.Body.Comment)
	if err != nil {
		mapped := moderationErrorModel(ctx, err)
		switch {
		case isModerationValidationError(err):
			return api.DecideReport400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse(mapped)}, nil
		case errors.Is(err, moderationmodel.ErrForbidden):
			return api.DecideReport403JSONResponse{ForbiddenJSONResponse: api.ForbiddenJSONResponse(mapped)}, nil
		case errors.Is(err, moderationmodel.ErrNotFound):
			return api.DecideReport404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse(mapped)}, nil
		case errors.Is(err, moderationmodel.ErrStateConflict):
			return api.DecideReport409JSONResponse{ConflictJSONResponse: api.ConflictJSONResponse(mapped)}, nil
		default:
			h.logger.Error("decide report", zap.Int64("actor_id", current.UserID), zap.Int64("report_id", request.ReportId), zap.Error(err))
			return api.DecideReport500JSONResponse{InternalErrorJSONResponse: api.InternalErrorJSONResponse(mapped)}, nil
		}
	}
	return api.DecideReport200JSONResponse(reportModel(report)), nil
}

// ListAdminAudit returns the newest-first audit log.
func (h *Handler) ListAdminAudit(ctx context.Context, request api.ListAdminAuditRequestObject) (api.ListAdminAuditResponseObject, error) {
	current, ok := session.Current(ctx)
	if !ok {
		return api.ListAdminAudit401JSONResponse{
			UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(sessionRequired(ctx)),
		}, nil
	}

	limit := 20
	if request.Params.Limit != nil {
		limit = *request.Params.Limit
	}
	var beforeID *int64
	if request.Params.Cursor != nil {
		parsed, err := strconv.ParseInt(*request.Params.Cursor, 10, 64)
		if err != nil || parsed <= 0 {
			return api.ListAdminAudit400JSONResponse{
				BadRequestJSONResponse: api.BadRequestJSONResponse(errorModel(ctx, "INVALID_CURSOR", "cursor must be a positive integer", nil)),
			}, nil
		}
		beforeID = &parsed
	}

	filter := moderationmodel.AuditFilter{}
	if request.Params.Action != nil {
		filter.Action = string(*request.Params.Action)
	}
	if request.Params.AdminId != nil {
		filter.AdminID = *request.Params.AdminId
	}
	if request.Params.TargetType != nil {
		filter.TargetType = *request.Params.TargetType
	}

	entries, next, err := h.moderation.ListAudit(ctx, current.UserID, filter, beforeID, limit)
	if err != nil {
		mapped := moderationErrorModel(ctx, err)
		switch {
		case isModerationValidationError(err):
			return api.ListAdminAudit400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse(mapped)}, nil
		case errors.Is(err, moderationmodel.ErrForbidden):
			return api.ListAdminAudit403JSONResponse{ForbiddenJSONResponse: api.ForbiddenJSONResponse(mapped)}, nil
		default:
			h.logger.Error("list admin audit", zap.Int64("actor_id", current.UserID), zap.Error(err))
			return api.ListAdminAudit500JSONResponse{InternalErrorJSONResponse: api.InternalErrorJSONResponse(mapped)}, nil
		}
	}

	response := api.AuditLogList{Entries: make([]api.AuditLogEntry, 0, len(entries))}
	for _, entry := range entries {
		response.Entries = append(response.Entries, auditModel(entry))
	}
	if next != nil {
		cursor := strconv.FormatInt(*next, 10)
		response.NextCursor = &cursor
	}
	return api.ListAdminAudit200JSONResponse(response), nil
}

func blockModel(block blocklistmodel.Block) api.Block {
	return api.Block{
		Id: block.ID,
		BlockedUser: api.UserSummary{
			Id:       block.BlockedUser.ID,
			Username: block.BlockedUser.Username,
		},
		CreatedAt: block.BlockedAt,
	}
}

func reportModel(report moderationmodel.Report) api.MessageReport {
	model := api.MessageReport{
		Id:        report.ID,
		Reporter:  api.UserSummary{Id: report.Reporter.ID, Username: report.Reporter.Username},
		MessageId: report.MessageID,
		Reason:    api.ReportReason(report.Reason),
		Status:    api.ReportStatus(report.Status),
		CreatedAt: report.CreatedAt,
		UpdatedAt: report.UpdatedAt,
	}
	if report.Comment != nil {
		model.Comment = report.Comment
	}
	if report.Assignee != nil {
		assignee := api.UserSummary{Id: report.Assignee.ID, Username: report.Assignee.Username}
		model.Assignee = &assignee
	}
	if report.DecisionComment != nil {
		model.DecisionComment = report.DecisionComment
	}
	return model
}

func userReportModel(report moderationmodel.UserReport) api.UserReport {
	return api.UserReport{
		Id:             report.ID,
		ReporterUserId: report.ReporterID,
		TargetUserId:   report.TargetID,
		ChainId:        report.ChainID,
		Reason:         api.UserReportReason(report.Reason),
		Comment:        report.Comment,
		CreatedAt:      report.CreatedAt,
	}
}

func reportDetailModel(detail moderationmodel.ReportDetail) api.ReportDetail {
	context := make([]api.ChatMessage, 0, len(detail.Context))
	for _, message := range detail.Context {
		context = append(context, chatMessageModel(message))
	}
	return api.ReportDetail{
		Report:          reportModel(detail.Report),
		ReportedMessage: chatMessageModel(detail.ReportedMessage),
		Context:         context,
	}
}

func auditModel(entry moderationmodel.AuditEntry) api.AuditLogEntry {
	return api.AuditLogEntry{
		Id:         entry.ID,
		Admin:      api.UserSummary{Id: entry.Admin.ID, Username: entry.Admin.Username},
		Action:     api.AuditAction(entry.Action),
		TargetType: entry.TargetType,
		TargetId:   entry.TargetID,
		Metadata:   entry.Metadata,
		CreatedAt:  entry.CreatedAt,
	}
}

func blocklistErrorModel(ctx context.Context, err error) api.Error {
	switch {
	case isBlocklistValidationError(err):
		return errorModel(ctx, "VALIDATION_ERROR", err.Error(), nil)
	case errors.Is(err, blocklistmodel.ErrSelfBlock):
		return errorModel(ctx, "SELF_BLOCK", "cannot block yourself", nil)
	case errors.Is(err, blocklistmodel.ErrTargetNotFound):
		return errorModel(ctx, "USER_NOT_FOUND", "target user not found", nil)
	case errors.Is(err, blocklistmodel.ErrForbidden):
		return errorModel(ctx, "FORBIDDEN", "block list is not available to the current user", nil)
	default:
		return errorModel(ctx, "INTERNAL_ERROR", "block list operation failed", nil)
	}
}

func moderationErrorModel(ctx context.Context, err error) api.Error {
	switch {
	case isModerationValidationError(err):
		return errorModel(ctx, "VALIDATION_ERROR", err.Error(), nil)
	case errors.Is(err, moderationmodel.ErrForbidden):
		return errorModel(ctx, "MODERATION_ACCESS_REQUIRED", "administrator access is required", nil)
	case errors.Is(err, moderationmodel.ErrNotFound):
		return errorModel(ctx, "NOT_FOUND", "report not found", nil)
	case errors.Is(err, moderationmodel.ErrSelfReport):
		return errorModel(ctx, "SELF_REPORT", "cannot report yourself or your own message", nil)
	case errors.Is(err, moderationmodel.ErrReportUnavailable):
		return errorModel(ctx, "NOT_FOUND", "message is not available to the current user", nil)
	case errors.Is(err, moderationmodel.ErrUserReportUnavailable):
		return errorModel(ctx, "NOT_FOUND", "chain is not available for this user report", nil)
	case errors.Is(err, moderationmodel.ErrStateConflict):
		return errorModel(ctx, "REPORT_STATE_CONFLICT", "report cannot change in its current state", nil)
	case errors.Is(err, moderationmodel.ErrAlreadyAssigned):
		return errorModel(ctx, "REPORT_ALREADY_ASSIGNED", "report is already assigned to another administrator", nil)
	default:
		return errorModel(ctx, "INTERNAL_ERROR", "moderation operation failed", nil)
	}
}

func isBlocklistValidationError(err error) bool {
	var validationError *blocklistmodel.ValidationError
	return errors.As(err, &validationError)
}

func isModerationValidationError(err error) bool {
	var validationError *moderationmodel.ValidationError
	return errors.As(err, &validationError)
}

func notificationModel(n notificationmodel.Notification) api.AppNotification {
	model := api.AppNotification{
		Id:        n.ID,
		Kind:      api.NotificationKind(n.Kind),
		Title:     n.Title,
		Text:      n.Text,
		CreatedAt: n.CreatedAt,
		Read:      n.Read,
	}
	if n.ChainID != nil {
		model.ChainId = n.ChainID
	}
	if n.ItemID != nil {
		model.ItemId = n.ItemID
	}
	return model
}

func itemModel(item items.Item) api.Item {
	return api.Item{
		Id:               item.ID,
		UserId:           item.UserID,
		OfferTitle:       item.OfferTitle,
		OfferDescription: item.OfferDescription,
		Wishes: func() []api.ItemWish {
			wishes := make([]api.ItemWish, 0, len(item.Wishes))
			for _, w := range item.Wishes {
				wishes = append(wishes, api.ItemWish{
					Id:          w.ID,
					CategoryId:  w.CategoryID,
					Description: w.Description,
				})
			}
			return wishes
		}(),
		ImageUrls:       append([]string{}, item.ImageURLs...),
		Status:          api.ItemStatus(item.Status),
		Condition:       api.ItemCondition(item.Condition),
		CategoryId:      item.OfferCategoryID,
		OfferCategoryId: item.OfferCategoryID,
		CreatedAt:       item.CreatedAt,
		UpdatedAt:       item.UpdatedAt,
	}
}

func chainModel(chain chains.Chain) api.Chain {
	participants := make([]api.ChainParticipant, 0, len(chain.Participants))
	for _, participant := range chain.Participants {
		p := api.ChainParticipant{
			User: api.UserSummary{
				Id:       participant.User.ID,
				Username: participant.User.Username,
			},
			GiveItem:                  itemModel(participant.GiveItem),
			ReceiveItem:               itemModel(participant.ReceiveItem),
			Status:                    api.ParticipantStatus(participant.Status),
			ReceiptConfirmed:          participant.ReceiptConfirmed,
			IncomingDeliveryStatus:    api.AdminDeliveryStatus(participant.IncomingDeliveryStatus),
			IncomingDeliveryUpdatedAt: participant.IncomingDeliveryUpdatedAt,
		}
		if participant.ReceiptConfirmedAt != nil {
			p.ReceiptConfirmedAt = participant.ReceiptConfirmedAt
		}
		participants = append(participants, p)
	}
	return api.Chain{
		Id:           chain.ID,
		Status:       api.ChainStatus(chain.Status),
		Participants: participants,
		CreatedAt:    chain.CreatedAt,
		ExpiresAt:    chain.ExpiresAt,
	}
}

func adminDeliveryModel(delivery adminmodel.Delivery) api.AdminDelivery {
	return api.AdminDelivery{
		Id:        delivery.ID,
		ChainId:   delivery.ChainID,
		ItemId:    delivery.ItemID,
		ItemTitle: delivery.ItemTitle,
		Sender: api.UserSummary{
			Id:       delivery.SenderID,
			Username: delivery.SenderUsername,
		},
		Recipient: api.UserSummary{
			Id:       delivery.RecipientID,
			Username: delivery.RecipientUsername,
		},
		Status:    api.AdminDeliveryStatus(delivery.Status),
		UpdatedAt: delivery.UpdatedAt,
	}
}

func adminChainSummaryModel(chain adminmodel.Chain) api.AdminChainSummary {
	return api.AdminChainSummary{Id: chain.ID, Status: api.AdminChainStatus(chain.Status), ParticipantCount: chain.ParticipantCount, ReceivedCount: chain.ReceivedCount, CreatedAt: chain.CreatedAt, UpdatedAt: chain.UpdatedAt}
}

func adminChainModel(chain adminmodel.Chain) api.AdminChain {
	deliveries := make([]api.AdminDelivery, 0, len(chain.Deliveries))
	for _, delivery := range chain.Deliveries {
		deliveries = append(deliveries, adminDeliveryModel(delivery))
	}
	return api.AdminChain{Id: chain.ID, Status: api.AdminChainStatus(chain.Status), ParticipantCount: chain.ParticipantCount, ReceivedCount: chain.ReceivedCount, CreatedAt: chain.CreatedAt, UpdatedAt: chain.UpdatedAt, Deliveries: deliveries}
}

func chatMessageModel(message chatmodel.Message) api.ChatMessage {
	return api.ChatMessage{
		Id:     message.ID,
		ItemId: message.ItemID,
		Sender: api.UserSummary{
			Id:       message.Sender.ID,
			Username: message.Sender.Username,
		},
		Recipient: api.UserSummary{
			Id:       message.Recipient.ID,
			Username: message.Recipient.Username,
		},
		ClientMessageId: message.ClientMessageID,
		Text:            message.Text,
		CreatedAt:       message.CreatedAt,
	}
}

func chatThreadModel(thread chatmodel.Thread) api.ChatThread {
	result := api.ChatThread{
		Item: chatItemSummaryModel(thread.Item),
		Counterpart: api.UserSummary{
			Id:       thread.Counterpart.ID,
			Username: thread.Counterpart.Username,
		},
		HasUnread:   thread.UnreadCount > 0,
		UnreadCount: thread.UnreadCount,
	}
	if thread.LastMessage != nil {
		message := chatMessageModel(*thread.LastMessage)
		result.LastMessage = &message
	}
	return result
}

func chatItemSummaryModel(item chatmodel.ItemSummary) api.ChatItemSummary {
	result := api.ChatItemSummary{Id: item.ID, Title: item.Title}
	if item.ImageURL != "" {
		result.ImageUrl = &item.ImageURL
	}
	return result
}

func chatReadStateModel(readState chatmodel.ReadState) api.ChatReadState {
	return api.ChatReadState{
		ItemId:            readState.ItemID,
		CounterpartId:     readState.CounterpartID,
		LastReadMessageId: readState.LastReadMessageID,
		UnreadCount:       readState.UnreadCount,
	}
}

func sessionModel(current session.Session, user users.User) api.Session {
	return api.Session{
		ExpiresAt: current.ExpiresAt,
		User: api.CurrentUser{
			Id:        user.ID,
			Username:  user.Username,
			Phone:     user.Phone,
			Role:      api.UserRole(user.Role),
			CreatedAt: user.CreatedAt,
		},
	}
}

func userProfileModel(user users.User, stats reputationmodel.Stats) api.UserProfile {
	profile := api.UserProfile{
		Id:                 user.ID,
		Username:           user.Username,
		CreatedAt:          user.CreatedAt,
		CompletedExchanges: stats.CompletedExchanges,
		Rating:             stats.Rating,
		ReviewsCount:       stats.ReviewsCount,
	}
	if user.AvatarURL != "" {
		profile.AvatarUrl = &user.AvatarURL
	}
	return profile
}

func userReviewModel(review reputationmodel.Review) api.UserReview {
	return api.UserReview{
		Id:      review.ID,
		ChainId: review.ChainID,
		Author: api.UserSummary{
			Id:       review.Author.ID,
			Username: review.Author.Username,
		},
		TargetUserId: review.TargetUserID,
		Rating:       review.Rating,
		Text:         review.Text,
		CreatedAt:    review.CreatedAt,
	}
}

func funnelMetricsModel(metrics metricsmodel.Funnel) api.FunnelMetrics {
	reasons := make([]api.FunnelRejectionReason, 0, len(metrics.RejectionReasons))
	for _, reason := range metrics.RejectionReasons {
		reasons = append(reasons, api.FunnelRejectionReason{
			Reason: api.FunnelRejectionReasonReason(reason.Reason),
			Count:  reason.Count,
		})
	}
	return api.FunnelMetrics{
		GeneratedAt:                    metrics.GeneratedAt,
		EligibleItems:                  metrics.EligibleItems,
		ItemsWithChain:                 metrics.ItemsWithChain,
		ItemsWithChainRate:             metrics.ItemsWithChainRate,
		AverageTimeToFirstChainSeconds: metrics.AverageTimeToFirstChainSeconds,
		DecidedChains:                  metrics.DecidedChains,
		AcceptedChains:                 metrics.AcceptedChains,
		AcceptanceRate:                 metrics.AcceptanceRate,
		CompletedChains:                metrics.CompletedChains,
		DeliveryCompletionRate:         metrics.DeliveryCompletionRate,
		RejectionReasons:               reasons,
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

func isAdminValidationError(err error) bool {
	var validationError *adminmodel.ValidationError
	return errors.As(err, &validationError)
}

func isChatValidationError(err error) bool {
	var validationError *chatmodel.ValidationError
	return errors.As(err, &validationError)
}

func chatErrorModel(ctx context.Context, err error) api.Error {
	switch {
	case isChatValidationError(err):
		return errorModel(ctx, "VALIDATION_ERROR", err.Error(), nil)
	case errors.Is(err, chatmodel.ErrForbidden):
		return errorModel(ctx, "CHAT_FORBIDDEN", "chat is not available to the current user", nil)
	case errors.Is(err, chatmodel.ErrItemNotFound):
		return errorModel(ctx, "ITEM_NOT_FOUND", "chat item not found", nil)
	case errors.Is(err, chatmodel.ErrThreadNotFound):
		return errorModel(ctx, "CHAT_THREAD_NOT_FOUND", "chat thread not found", nil)
	case errors.Is(err, chatmodel.ErrMessageNotFound):
		return errorModel(ctx, "CHAT_MESSAGE_NOT_FOUND", "chat message not found in this thread", nil)
	case errors.Is(err, chatmodel.ErrIdempotencyConflict):
		return errorModel(ctx, "CLIENT_MESSAGE_ID_CONFLICT", "clientMessageId was already used with different text", nil)
	default:
		return errorModel(ctx, "INTERNAL_ERROR", "chat operation failed", nil)
	}
}

func adminErrorModel(ctx context.Context, err error) api.Error {
	switch {
	case isAdminValidationError(err):
		return errorModel(ctx, "VALIDATION_ERROR", err.Error(), nil)
	case errors.Is(err, adminmodel.ErrForbidden):
		return errorModel(ctx, "ADMIN_ACCESS_REQUIRED", "pickup-point administrator access is required", nil)
	case errors.Is(err, adminmodel.ErrReceiptForbidden):
		return errorModel(ctx, "RECEIPT_FORBIDDEN", "only the recipient can confirm this delivery", nil)
	case errors.Is(err, adminmodel.ErrChainNotFound):
		return errorModel(ctx, "CHAIN_NOT_FOUND", "chain not found", nil)
	case errors.Is(err, adminmodel.ErrDeliveryNotFound):
		return errorModel(ctx, "DELIVERY_NOT_FOUND", "delivery not found", nil)
	case errors.Is(err, adminmodel.ErrTransitionConflict):
		return errorModel(ctx, "DELIVERY_STATE_CONFLICT", "delivery cannot enter the requested status from its current state", nil)
	default:
		return errorModel(ctx, "INTERNAL_ERROR", "admin operation failed", nil)
	}
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
