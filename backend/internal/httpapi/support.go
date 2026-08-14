package httpapi

import (
	"context"
	"errors"
	"time"

	"swap-chain/internal/api"
	"swap-chain/internal/session"
	supportmodel "swap-chain/modules/support/model"
)

func supportMessageModel(m supportmodel.Message) api.SupportMessage {
	out := api.SupportMessage{Id: m.ID, ThreadId: m.ThreadID, SenderType: api.SupportSenderType(m.SenderType), ClientMessageId: m.ClientMessageID, Text: m.Text, CreatedAt: m.CreatedAt}
	if m.Sender != nil {
		out.Sender = &api.UserSummary{Id: m.Sender.ID, Username: m.Sender.Username}
	}
	return out
}
func supportThreadModel(t supportmodel.Thread) api.SupportThread {
	out := api.SupportThread{Id: t.ID, User: api.UserSummary{Id: t.User.ID, Username: t.User.Username}, Moderators: make([]api.UserSummary, 0, len(t.Moderators)), UnreadCount: t.UnreadCount, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt}
	for _, m := range t.Moderators {
		out.Moderators = append(out.Moderators, api.UserSummary{Id: m.ID, Username: m.Username})
	}
	if t.LastMessage != nil {
		x := supportMessageModel(*t.LastMessage)
		out.LastMessage = &x
	}
	return out
}
func supportMessagesModel(messages []supportmodel.Message) api.SupportMessageList {
	out := api.SupportMessageList{Messages: make([]api.SupportMessage, 0, len(messages))}
	for _, m := range messages {
		out.Messages = append(out.Messages, supportMessageModel(m))
	}
	if len(messages) > 0 {
		x := messages[len(messages)-1].ID
		out.NextAfterId = &x
	}
	return out
}
func supportReadModel(s supportmodel.ReadState) api.SupportReadState {
	return api.SupportReadState{ThreadId: s.ThreadID, LastReadMessageId: s.LastReadMessageID, UnreadCount: s.UnreadCount}
}
func supportError(ctx context.Context, err error) api.Error {
	var v *supportmodel.ValidationError
	switch {
	case errors.As(err, &v):
		return errorModel(ctx, "VALIDATION_ERROR", v.Error(), map[string]any{v.Field: v.Message})
	case errors.Is(err, supportmodel.ErrForbidden):
		return errorModel(ctx, "FORBIDDEN", "support chat is not available", nil)
	case errors.Is(err, supportmodel.ErrNotFound):
		return errorModel(ctx, "NOT_FOUND", "support thread or message not found", nil)
	case errors.Is(err, supportmodel.ErrNotJoined):
		return errorModel(ctx, "MODERATOR_NOT_JOINED", "connect to the support thread before sending", nil)
	case errors.Is(err, supportmodel.ErrIdempotencyConflict):
		return errorModel(ctx, "IDEMPOTENCY_CONFLICT", err.Error(), nil)
	default:
		return errorModel(ctx, "INTERNAL_ERROR", "support operation failed", nil)
	}
}
func currentSupport(ctx context.Context) (session.Session, bool) { return session.Current(ctx) }
func params(after *int64, limit, wait *int) (int64, int, time.Duration) {
	a, l, w := int64(0), 50, 0
	if after != nil {
		a = *after
	}
	if limit != nil {
		l = *limit
	}
	if wait != nil {
		w = *wait
	}
	return a, l, time.Duration(w) * time.Second
}

func (h *Handler) GetSupportThread(ctx context.Context, _ api.GetSupportThreadRequestObject) (api.GetSupportThreadResponseObject, error) {
	cur, ok := currentSupport(ctx)
	if !ok {
		return api.GetSupportThread401JSONResponse{UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(sessionRequired(ctx))}, nil
	}
	t, err := h.support.UserThread(ctx, cur.UserID)
	if err != nil {
		return api.GetSupportThread500JSONResponse{InternalErrorJSONResponse: api.InternalErrorJSONResponse(supportError(ctx, err))}, nil
	}
	return api.GetSupportThread200JSONResponse(supportThreadModel(t)), nil
}
func (h *Handler) ListSupportMessages(ctx context.Context, r api.ListSupportMessagesRequestObject) (api.ListSupportMessagesResponseObject, error) {
	cur, ok := currentSupport(ctx)
	if !ok {
		return api.ListSupportMessages401JSONResponse{UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(sessionRequired(ctx))}, nil
	}
	t, err := h.support.UserThread(ctx, cur.UserID)
	if err != nil {
		return api.ListSupportMessages500JSONResponse{InternalErrorJSONResponse: api.InternalErrorJSONResponse(supportError(ctx, err))}, nil
	}
	a, l, w := params(r.Params.AfterId, r.Params.Limit, r.Params.WaitSeconds)
	m, err := h.support.List(ctx, cur.UserID, t.ID, a, l, w, false)
	if err != nil {
		var v *supportmodel.ValidationError
		if errors.As(err, &v) {
			return api.ListSupportMessages400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse(supportError(ctx, err))}, nil
		}
		return api.ListSupportMessages500JSONResponse{InternalErrorJSONResponse: api.InternalErrorJSONResponse(supportError(ctx, err))}, nil
	}
	return api.ListSupportMessages200JSONResponse(supportMessagesModel(m)), nil
}
func (h *Handler) SendSupportMessage(ctx context.Context, r api.SendSupportMessageRequestObject) (api.SendSupportMessageResponseObject, error) {
	cur, ok := currentSupport(ctx)
	if !ok {
		return api.SendSupportMessage401JSONResponse{UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(sessionRequired(ctx))}, nil
	}
	if r.Body == nil {
		return api.SendSupportMessage400JSONResponse{ValidationErrorJSONResponse: api.ValidationErrorJSONResponse(errorModel(ctx, "VALIDATION_ERROR", "body is required", nil))}, nil
	}
	t, err := h.support.UserThread(ctx, cur.UserID)
	if err != nil {
		return api.SendSupportMessage500JSONResponse{InternalErrorJSONResponse: api.InternalErrorJSONResponse(supportError(ctx, err))}, nil
	}
	m, created, err := h.support.Send(ctx, cur.UserID, t.ID, r.Body.ClientMessageId, r.Body.Text, false)
	if err != nil {
		if errors.Is(err, supportmodel.ErrIdempotencyConflict) {
			return api.SendSupportMessage409JSONResponse{ConflictJSONResponse: api.ConflictJSONResponse(supportError(ctx, err))}, nil
		}
		return api.SendSupportMessage400JSONResponse{ValidationErrorJSONResponse: api.ValidationErrorJSONResponse(supportError(ctx, err))}, nil
	}
	out := supportMessageModel(m)
	if created {
		return api.SendSupportMessage201JSONResponse(out), nil
	}
	return api.SendSupportMessage200JSONResponse(out), nil
}
func (h *Handler) MarkSupportRead(ctx context.Context, r api.MarkSupportReadRequestObject) (api.MarkSupportReadResponseObject, error) {
	cur, ok := currentSupport(ctx)
	if !ok {
		return api.MarkSupportRead401JSONResponse{UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(sessionRequired(ctx))}, nil
	}
	if r.Body == nil {
		return api.MarkSupportRead400JSONResponse{ValidationErrorJSONResponse: api.ValidationErrorJSONResponse(errorModel(ctx, "VALIDATION_ERROR", "body is required", nil))}, nil
	}
	t, err := h.support.UserThread(ctx, cur.UserID)
	if err != nil {
		return api.MarkSupportRead500JSONResponse{InternalErrorJSONResponse: api.InternalErrorJSONResponse(supportError(ctx, err))}, nil
	}
	state, err := h.support.MarkRead(ctx, cur.UserID, t.ID, r.Body.LastReadMessageId, false)
	if errors.Is(err, supportmodel.ErrNotFound) {
		return api.MarkSupportRead404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse(supportError(ctx, err))}, nil
	}
	if err != nil {
		return api.MarkSupportRead400JSONResponse{ValidationErrorJSONResponse: api.ValidationErrorJSONResponse(supportError(ctx, err))}, nil
	}
	return api.MarkSupportRead200JSONResponse(supportReadModel(state)), nil
}

func (h *Handler) ListAdminSupportThreads(ctx context.Context, r api.ListAdminSupportThreadsRequestObject) (api.ListAdminSupportThreadsResponseObject, error) {
	cur, ok := currentSupport(ctx)
	if !ok {
		return api.ListAdminSupportThreads401JSONResponse{UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(sessionRequired(ctx))}, nil
	}
	limit := 100
	if r.Params.Limit != nil {
		limit = *r.Params.Limit
	}
	threads, err := h.support.AdminThreads(ctx, cur.UserID, limit)
	if errors.Is(err, supportmodel.ErrForbidden) {
		return api.ListAdminSupportThreads403JSONResponse{ForbiddenJSONResponse: api.ForbiddenJSONResponse(supportError(ctx, err))}, nil
	}
	if err != nil {
		return api.ListAdminSupportThreads500JSONResponse{InternalErrorJSONResponse: api.InternalErrorJSONResponse(supportError(ctx, err))}, nil
	}
	out := api.ListAdminSupportThreads200JSONResponse{Threads: make([]api.SupportThread, 0, len(threads))}
	for _, t := range threads {
		out.Threads = append(out.Threads, supportThreadModel(t))
	}
	return out, nil
}
func (h *Handler) GetAdminSupportThread(ctx context.Context, r api.GetAdminSupportThreadRequestObject) (api.GetAdminSupportThreadResponseObject, error) {
	cur, ok := currentSupport(ctx)
	if !ok {
		return api.GetAdminSupportThread401JSONResponse{UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(sessionRequired(ctx))}, nil
	}
	t, err := h.support.AdminThread(ctx, cur.UserID, r.ThreadId)
	if errors.Is(err, supportmodel.ErrForbidden) {
		return api.GetAdminSupportThread403JSONResponse{ForbiddenJSONResponse: api.ForbiddenJSONResponse(supportError(ctx, err))}, nil
	}
	if errors.Is(err, supportmodel.ErrNotFound) {
		return api.GetAdminSupportThread404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse(supportError(ctx, err))}, nil
	}
	if err != nil {
		return api.GetAdminSupportThread500JSONResponse{InternalErrorJSONResponse: api.InternalErrorJSONResponse(supportError(ctx, err))}, nil
	}
	return api.GetAdminSupportThread200JSONResponse(supportThreadModel(t)), nil
}
func (h *Handler) JoinSupportThread(ctx context.Context, r api.JoinSupportThreadRequestObject) (api.JoinSupportThreadResponseObject, error) {
	cur, ok := currentSupport(ctx)
	if !ok {
		return api.JoinSupportThread401JSONResponse{UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(sessionRequired(ctx))}, nil
	}
	t, _, err := h.support.Join(ctx, cur.UserID, r.ThreadId)
	if errors.Is(err, supportmodel.ErrForbidden) {
		return api.JoinSupportThread403JSONResponse{ForbiddenJSONResponse: api.ForbiddenJSONResponse(supportError(ctx, err))}, nil
	}
	if errors.Is(err, supportmodel.ErrNotFound) {
		return api.JoinSupportThread404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse(supportError(ctx, err))}, nil
	}
	if err != nil {
		return api.JoinSupportThread500JSONResponse{InternalErrorJSONResponse: api.InternalErrorJSONResponse(supportError(ctx, err))}, nil
	}
	return api.JoinSupportThread200JSONResponse(supportThreadModel(t)), nil
}
func (h *Handler) LeaveSupportThread(ctx context.Context, r api.LeaveSupportThreadRequestObject) (api.LeaveSupportThreadResponseObject, error) {
	cur, ok := currentSupport(ctx)
	if !ok {
		return api.LeaveSupportThread401JSONResponse{UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(sessionRequired(ctx))}, nil
	}
	t, _, err := h.support.Leave(ctx, cur.UserID, r.ThreadId)
	if errors.Is(err, supportmodel.ErrForbidden) {
		return api.LeaveSupportThread403JSONResponse{ForbiddenJSONResponse: api.ForbiddenJSONResponse(supportError(ctx, err))}, nil
	}
	if errors.Is(err, supportmodel.ErrNotFound) {
		return api.LeaveSupportThread404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse(supportError(ctx, err))}, nil
	}
	if err != nil {
		return api.LeaveSupportThread500JSONResponse{InternalErrorJSONResponse: api.InternalErrorJSONResponse(supportError(ctx, err))}, nil
	}
	return api.LeaveSupportThread200JSONResponse(supportThreadModel(t)), nil
}
func (h *Handler) ListAdminSupportMessages(ctx context.Context, r api.ListAdminSupportMessagesRequestObject) (api.ListAdminSupportMessagesResponseObject, error) {
	cur, ok := currentSupport(ctx)
	if !ok {
		return api.ListAdminSupportMessages401JSONResponse{UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(sessionRequired(ctx))}, nil
	}
	a, l, w := params(r.Params.AfterId, r.Params.Limit, r.Params.WaitSeconds)
	m, err := h.support.List(ctx, cur.UserID, r.ThreadId, a, l, w, true)
	if errors.Is(err, supportmodel.ErrForbidden) {
		return api.ListAdminSupportMessages403JSONResponse{ForbiddenJSONResponse: api.ForbiddenJSONResponse(supportError(ctx, err))}, nil
	}
	if errors.Is(err, supportmodel.ErrNotFound) {
		return api.ListAdminSupportMessages404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse(supportError(ctx, err))}, nil
	}
	if err != nil {
		return api.ListAdminSupportMessages400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse(supportError(ctx, err))}, nil
	}
	return api.ListAdminSupportMessages200JSONResponse(supportMessagesModel(m)), nil
}
func (h *Handler) SendAdminSupportMessage(ctx context.Context, r api.SendAdminSupportMessageRequestObject) (api.SendAdminSupportMessageResponseObject, error) {
	cur, ok := currentSupport(ctx)
	if !ok {
		return api.SendAdminSupportMessage401JSONResponse{UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(sessionRequired(ctx))}, nil
	}
	if r.Body == nil {
		return api.SendAdminSupportMessage400JSONResponse{ValidationErrorJSONResponse: api.ValidationErrorJSONResponse(errorModel(ctx, "VALIDATION_ERROR", "body is required", nil))}, nil
	}
	m, created, err := h.support.Send(ctx, cur.UserID, r.ThreadId, r.Body.ClientMessageId, r.Body.Text, true)
	switch {
	case errors.Is(err, supportmodel.ErrForbidden), errors.Is(err, supportmodel.ErrNotJoined):
		return api.SendAdminSupportMessage403JSONResponse{ForbiddenJSONResponse: api.ForbiddenJSONResponse(supportError(ctx, err))}, nil
	case errors.Is(err, supportmodel.ErrNotFound):
		return api.SendAdminSupportMessage404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse(supportError(ctx, err))}, nil
	case errors.Is(err, supportmodel.ErrIdempotencyConflict):
		return api.SendAdminSupportMessage409JSONResponse{ConflictJSONResponse: api.ConflictJSONResponse(supportError(ctx, err))}, nil
	case err != nil:
		return api.SendAdminSupportMessage400JSONResponse{ValidationErrorJSONResponse: api.ValidationErrorJSONResponse(supportError(ctx, err))}, nil
	}
	out := supportMessageModel(m)
	if created {
		return api.SendAdminSupportMessage201JSONResponse(out), nil
	}
	return api.SendAdminSupportMessage200JSONResponse(out), nil
}
func (h *Handler) MarkAdminSupportRead(ctx context.Context, r api.MarkAdminSupportReadRequestObject) (api.MarkAdminSupportReadResponseObject, error) {
	cur, ok := currentSupport(ctx)
	if !ok {
		return api.MarkAdminSupportRead401JSONResponse{UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(sessionRequired(ctx))}, nil
	}
	if r.Body == nil {
		return api.MarkAdminSupportRead400JSONResponse{ValidationErrorJSONResponse: api.ValidationErrorJSONResponse(errorModel(ctx, "VALIDATION_ERROR", "body required", nil))}, nil
	}
	state, err := h.support.MarkRead(ctx, cur.UserID, r.ThreadId, r.Body.LastReadMessageId, true)
	if errors.Is(err, supportmodel.ErrForbidden) {
		return api.MarkAdminSupportRead403JSONResponse{ForbiddenJSONResponse: api.ForbiddenJSONResponse(supportError(ctx, err))}, nil
	}
	if errors.Is(err, supportmodel.ErrNotFound) {
		return api.MarkAdminSupportRead404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse(supportError(ctx, err))}, nil
	}
	if err != nil {
		return api.MarkAdminSupportRead400JSONResponse{ValidationErrorJSONResponse: api.ValidationErrorJSONResponse(supportError(ctx, err))}, nil
	}
	return api.MarkAdminSupportRead200JSONResponse(supportReadModel(state)), nil
}
