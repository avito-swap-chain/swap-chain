package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"swap-chain/modules/moderation/model"
)

type fakeRepository struct {
	report model.Report
	detail model.ReportDetail
	err    error
}

func (f *fakeRepository) CreateReport(_ context.Context, _, _ int64, _ string, _ *string) (model.Report, bool, error) {
	return f.report, true, f.err
}

func (f *fakeRepository) CreateUserReport(_ context.Context, _, _ int64, _ *int64, _ string, _ *string) (model.UserReport, bool, error) {
	return model.UserReport{}, true, f.err
}

func (f *fakeRepository) ListReports(_ context.Context, _ int64, _ model.ReportFilter, _ int64, _ int) ([]model.Report, *int64, error) {
	return nil, nil, f.err
}

func (f *fakeRepository) GetReport(_ context.Context, _, _ int64) (model.ReportDetail, error) {
	return f.detail, f.err
}

func (f *fakeRepository) Assign(_ context.Context, _, _ int64) (model.Report, error) {
	return f.report, f.err
}

func (f *fakeRepository) Decide(_ context.Context, _, _ int64, _, _ string) (model.Report, error) {
	return f.report, f.err
}

func (f *fakeRepository) ListAudit(_ context.Context, _ int64, _ model.AuditFilter, _ *int64, _ int) ([]model.AuditEntry, *int64, error) {
	return nil, nil, f.err
}

func newTestService() *Moderation {
	return &Moderation{repository: &fakeRepository{}, cfg: ModerationConfig{MaxListLimit: 100}}
}

func TestCreateReportValidation(t *testing.T) {
	service := newTestService()
	ctx := context.Background()

	if _, _, err := service.CreateReport(ctx, 0, 1, model.ReasonSpam, ""); !errors.Is(err, model.ErrForbidden) {
		t.Fatalf("CreateReport(reporter=0) error = %v, want ErrForbidden", err)
	}
	if _, _, err := service.CreateReport(ctx, 1, 0, model.ReasonSpam, ""); !isValidationError(err) {
		t.Fatalf("CreateReport(message=0) error = %v, want ValidationError", err)
	}
	if _, _, err := service.CreateReport(ctx, 1, 1, "invalid", ""); !isValidationError(err) {
		t.Fatalf("CreateReport(reason=invalid) error = %v, want ValidationError", err)
	}
	if _, _, err := service.CreateReport(ctx, 1, 1, model.ReasonOther, ""); !isValidationError(err) {
		t.Fatalf("CreateReport(other, no comment) error = %v, want ValidationError", err)
	}
	if _, _, err := service.CreateReport(ctx, 1, 1, model.ReasonSpam, strings.Repeat("x", model.MaxCommentLength+1)); !isValidationError(err) {
		t.Fatalf("CreateReport(long comment) error = %v, want ValidationError", err)
	}
	if _, _, err := service.CreateReport(ctx, 1, 1, model.ReasonSpam, ""); err != nil {
		t.Fatalf("CreateReport(spam, no comment) error = %v", err)
	}
	if _, _, err := service.CreateReport(ctx, 1, 1, model.ReasonOther, "spammy"); err != nil {
		t.Fatalf("CreateReport(other, comment) error = %v", err)
	}
}

func TestCreateUserReportValidation(t *testing.T) {
	service := newTestService()
	ctx := context.Background()

	if _, _, err := service.CreateUserReport(ctx, 0, 2, nil, model.ReasonRude, ""); !errors.Is(err, model.ErrForbidden) {
		t.Fatalf("CreateUserReport(reporter=0) error = %v, want ErrForbidden", err)
	}
	if _, _, err := service.CreateUserReport(ctx, 1, 1, nil, model.ReasonRude, ""); !errors.Is(err, model.ErrSelfReport) {
		t.Fatalf("CreateUserReport(self) error = %v, want ErrSelfReport", err)
	}
	if _, _, err := service.CreateUserReport(ctx, 1, 2, nil, "invalid", ""); !isValidationError(err) {
		t.Fatalf("CreateUserReport(reason=invalid) error = %v, want ValidationError", err)
	}
	if _, _, err := service.CreateUserReport(ctx, 1, 2, nil, model.ReasonOther, ""); !isValidationError(err) {
		t.Fatalf("CreateUserReport(other, no comment) error = %v, want ValidationError", err)
	}
	chainID := int64(0)
	if _, _, err := service.CreateUserReport(ctx, 1, 2, &chainID, model.ReasonFraud, ""); !isValidationError(err) {
		t.Fatalf("CreateUserReport(chain=0) error = %v, want ValidationError", err)
	}
	if _, _, err := service.CreateUserReport(ctx, 1, 2, nil, model.ReasonItem, "broken listing"); err != nil {
		t.Fatalf("CreateUserReport(valid) error = %v", err)
	}
}

func TestDecideValidation(t *testing.T) {
	service := newTestService()
	ctx := context.Background()

	if _, err := service.Decide(ctx, 1, 1, "invalid", "rationale"); !isValidationError(err) {
		t.Fatalf("Decide(decision=invalid) error = %v, want ValidationError", err)
	}
	if _, err := service.Decide(ctx, 1, 1, model.DecisionResolved, "   "); !isValidationError(err) {
		t.Fatalf("Decide(blank comment) error = %v, want ValidationError", err)
	}
	if _, err := service.Decide(ctx, 1, 1, model.DecisionRejected, strings.Repeat("x", model.MaxCommentLength+1)); !isValidationError(err) {
		t.Fatalf("Decide(long comment) error = %v, want ValidationError", err)
	}
	if _, err := service.Decide(ctx, 1, 1, model.DecisionResolved, "rationale"); err != nil {
		t.Fatalf("Decide(valid) error = %v", err)
	}
}

func TestListReportsFilterValidation(t *testing.T) {
	service := newTestService()
	ctx := context.Background()

	if _, _, err := service.ListReports(ctx, 1, model.ReportFilter{Status: "bogus"}, 0, 10); !isValidationError(err) {
		t.Fatalf("ListReports(bad status) error = %v, want ValidationError", err)
	}
	if _, _, err := service.ListReports(ctx, 1, model.ReportFilter{Reason: "bogus"}, 0, 10); !isValidationError(err) {
		t.Fatalf("ListReports(bad reason) error = %v, want ValidationError", err)
	}
	if _, _, err := service.ListReports(ctx, 1, model.ReportFilter{Unassigned: true, AssigneeID: 5}, 0, 10); !isValidationError(err) {
		t.Fatalf("ListReports(unassigned+assignee) error = %v, want ValidationError", err)
	}
	if _, _, err := service.ListReports(ctx, 1, model.ReportFilter{Status: model.ReportOpen}, 0, 10); err != nil {
		t.Fatalf("ListReports(valid) error = %v", err)
	}
}

func TestListAuditValidation(t *testing.T) {
	service := newTestService()
	ctx := context.Background()

	if _, _, err := service.ListAudit(ctx, 1, model.AuditFilter{Action: "bogus"}, nil, 10); !isValidationError(err) {
		t.Fatalf("ListAudit(bad action) error = %v, want ValidationError", err)
	}
	if _, _, err := service.ListAudit(ctx, 1, model.AuditFilter{Action: model.ActionReportAssigned}, nil, 10); err != nil {
		t.Fatalf("ListAudit(valid) error = %v", err)
	}
}

func isValidationError(err error) bool {
	var validationError *model.ValidationError
	return errors.As(err, &validationError)
}
