package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	"go.uber.org/zap"

	"swap-chain/internal/api"
	"swap-chain/internal/media"
	"swap-chain/internal/session"
	analyzemodel "swap-chain/modules/analyze/model"
)

const (
	maxPendingVisionJobs  = 4
	visionAnalysisTimeout = 90 * time.Second
)

// VisionService describes the photo analysis capability needed by the vision endpoint.
type VisionService interface {
	DescribeImage(ctx context.Context, imageBytes []byte) (*analyzemodel.VisualAnalysis, error)
}

// AnalyzePhoto triggers async photo analysis via AI and delivers results through SSE.
func (h *Handler) AnalyzePhoto(ctx context.Context, request api.AnalyzePhotoRequestObject) (api.AnalyzePhotoResponseObject, error) {
	current, ok := session.Current(ctx)
	if !ok {
		return api.AnalyzePhoto401JSONResponse{
			UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(sessionRequired(ctx)),
		}, nil
	}
	if request.Body == nil || strings.TrimSpace(request.Body.ImageUrl) == "" {
		return api.AnalyzePhoto400JSONResponse{
			BadRequestJSONResponse: api.BadRequestJSONResponse(errorModel(ctx, "INVALID_REQUEST", "JSON body with imageUrl is required", nil)),
		}, nil
	}

	if h.vision == nil {
		return api.AnalyzePhoto503JSONResponse(errorModel(ctx, "VISION_UNAVAILABLE", "photo analysis is not configured", nil)), nil
	}
	select {
	case h.visionJobs <- struct{}{}:
	default:
		return api.AnalyzePhoto429JSONResponse(errorModel(ctx, "VISION_BUSY", "photo analysis queue is full; retry later", nil)), nil
	}
	jobStarted := false
	defer func() {
		if !jobStarted {
			<-h.visionJobs
		}
	}()

	imageURL := request.Body.ImageUrl
	if !media.IsPublicURL(imageURL) {
		return api.AnalyzePhoto400JSONResponse{
			BadRequestJSONResponse: api.BadRequestJSONResponse(errorModel(ctx, "INVALID_IMAGE_URL", "imageUrl must be a valid /api/v1/media/* URL", nil)),
		}, nil
	}

	objectKey := strings.TrimPrefix(imageURL, "/api/v1/media/")
	stored, err := h.media.Open(ctx, objectKey)
	if errors.Is(err, media.ErrNotFound) {
		return api.AnalyzePhoto404JSONResponse{
			NotFoundJSONResponse: api.NotFoundJSONResponse(errorModel(ctx, "MEDIA_NOT_FOUND", "image not found", nil)),
		}, nil
	}
	if err != nil {
		h.logger.Error("open media for vision", zap.String("object_key", objectKey), zap.Error(err))
		return api.AnalyzePhoto500JSONResponse{
			InternalErrorJSONResponse: api.InternalErrorJSONResponse(errorModel(ctx, "INTERNAL_ERROR", "failed to load image for analysis", nil)),
		}, nil
	}
	defer func() { _ = stored.Body.Close() }()
	if stored.ContentType != "image/jpeg" && stored.ContentType != "image/png" {
		return api.AnalyzePhoto415JSONResponse(errorModel(ctx, "UNSUPPORTED_VISION_MEDIA_TYPE", "photo analysis supports only JPEG and PNG images", nil)), nil
	}
	photoBytes, readErr := io.ReadAll(stored.Body)
	if readErr != nil {
		h.logger.Error("read media for vision", zap.String("object_key", objectKey), zap.Error(readErr))
		return api.AnalyzePhoto500JSONResponse{
			InternalErrorJSONResponse: api.InternalErrorJSONResponse(errorModel(ctx, "INTERNAL_ERROR", "failed to read image for analysis", nil)),
		}, nil
	}

	userID := current.UserID
	jobStarted = true
	go func() {
		defer func() { <-h.visionJobs }()
		analysisCtx, cancel := context.WithTimeout(context.Background(), visionAnalysisTimeout)
		defer cancel()
		h.runPhotoAnalysis(analysisCtx, userID, imageURL, photoBytes)
	}()

	return api.AnalyzePhoto202JSONResponse{
		ImageUrl: imageURL,
		Status:   "accepted",
	}, nil
}

func (h *Handler) runPhotoAnalysis(ctx context.Context, userID int64, imageURL string, photoBytes []byte) {
	analysis, err := h.vision.DescribeImage(ctx, photoBytes)
	if err != nil {
		h.logger.Error("vision analysis failed",
			zap.Int64("user_id", userID),
			zap.String("image_url", imageURL),
			zap.Error(err))
		h.events.PublishToUser(userID, "vision.analysis.failed", imageURL, map[string]any{
			"error": "photo analysis failed",
		})
		return
	}

	result := map[string]any{
		"imageUrl":               imageURL,
		"suggestedCategory":      analysis.SuggestedCategory,
		"visualQuality":          string(analysis.VisualQuality),
		"qualityScore":           analysis.QualityScore,
		"marketplaceDescription": analysis.MarketplaceDescription,
	}

	resultJSON, _ := json.Marshal(result)
	h.logger.Info("vision analysis completed",
		zap.Int64("user_id", userID),
		zap.String("image_url", imageURL),
		zap.String("result", string(resultJSON)))

	h.events.PublishToUser(userID, "vision.analysis.completed", imageURL, result)
}
