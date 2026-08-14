package items

import (
	"context"
	"reflect"
	"testing"
	"time"

	"go.uber.org/zap"
)

type orchestrationRepoStub struct {
	item        Item
	update      updateResult
	page        []Item
	next        *int64
	resolveCall bool
}

func (r *orchestrationRepoStub) Create(context.Context, int64, CreateInput) (Item, error) {
	return r.item, nil
}
func (r *orchestrationRepoStub) Update(context.Context, int64, int64, UpdateInput) (updateResult, error) {
	return r.update, nil
}
func (r *orchestrationRepoStub) ResolveCategories(context.Context, int64, int64, CategoryDecisionInput) (Item, error) {
	r.resolveCall = true
	return r.item, nil
}
func (r *orchestrationRepoStub) Get(context.Context, int64) (Item, error) { return r.item, nil }
func (r *orchestrationRepoStub) ListByUser(context.Context, int64, int64, int) ([]Item, *int64, error) {
	return r.page, r.next, nil
}

func TestPostgresServicePublishesRejectedChainsOnWithdrawal(t *testing.T) {
	t.Parallel()
	repo := &orchestrationRepoStub{update: updateResult{
		Item:       Item{ID: 5, UserID: 7, Status: "WITHDRAWN"},
		Rejections: []chainRejection{{ChainID: 12, UserIDs: []int64{7, 9}}},
	}}
	type event struct {
		userID int64
		kind   string
		id     string
		data   map[string]any
	}
	var events []event
	svc := newPostgresService(repo, fakeAnalyzer{analyze: func(context.Context, int64) error { return nil }}, func(userID int64, kind, id string, data map[string]any) {
		events = append(events, event{userID, kind, id, data})
	}, zap.NewNop(), time.Second)
	t.Cleanup(svc.Close)

	item, err := svc.Update(context.Background(), 7, 5, UpdateInput{Withdraw: true})
	if err != nil || item.Status != "WITHDRAWN" {
		t.Fatalf("Update() = %+v, %v", item, err)
	}
	if len(events) != 3 {
		t.Fatalf("events = %v, want two chain rejections and one item update", events)
	}
	for _, got := range events[:2] {
		if got.kind != "chain.rejected" || got.id != "12" || got.data["reason"] != "item_withdrawn" || got.data["itemId"] != int64(5) {
			t.Fatalf("rejection event = %+v", got)
		}
	}
	if events[2].kind != "item.status.updated" || events[2].userID != 7 {
		t.Fatalf("item event = %+v", events[2])
	}
}

func TestPostgresServiceDelegatesReadAndCategoryOperations(t *testing.T) {
	t.Parallel()
	next := int64(11)
	repo := &orchestrationRepoStub{
		item: Item{ID: 5, UserID: 7, Status: "MATCHING"},
		page: []Item{{ID: 5}},
		next: &next,
	}
	var eventTypes []string
	svc := newPostgresService(repo, fakeAnalyzer{analyze: func(context.Context, int64) error { return nil }}, func(_ int64, kind, _ string, _ map[string]any) {
		eventTypes = append(eventTypes, kind)
	}, zap.NewNop(), time.Second)
	t.Cleanup(svc.Close)

	if item, err := svc.Get(context.Background(), 5); err != nil || item.ID != 5 {
		t.Fatalf("Get() = %+v, %v", item, err)
	}
	page, gotNext, err := svc.ListByUser(context.Background(), 7, 0, 10)
	if err != nil || !reflect.DeepEqual(page, repo.page) || gotNext == nil || *gotNext != next {
		t.Fatalf("ListByUser() = %v, %v, %v", page, gotNext, err)
	}
	categoryID := int32(3)
	item, err := svc.ResolveCategories(context.Background(), 7, 5, CategoryDecisionInput{OfferCategoryID: &categoryID})
	if err != nil || item.Status != "MATCHING" || !repo.resolveCall {
		t.Fatalf("ResolveCategories() = %+v, %v, called=%v", item, err, repo.resolveCall)
	}
	if !reflect.DeepEqual(eventTypes, []string{"item.status.updated"}) {
		t.Fatalf("events = %v", eventTypes)
	}
}
