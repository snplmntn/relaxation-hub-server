package service

import (
	"context"
	"errors"
	"testing"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

type stubSupportTicketRepo struct {
	listFn func(ctx context.Context, userID *int64, status *string, limit, offset int) ([]model.SupportTicket, int, error)
}

func (s *stubSupportTicketRepo) Create(ctx context.Context, ticket *model.SupportTicket) (*model.SupportTicket, error) {
	panic("unexpected call to Create")
}

func (s *stubSupportTicketRepo) CreateAttachments(ctx context.Context, attachments []model.SupportTicketAttachment) error {
	panic("unexpected call to CreateAttachments")
}

func (s *stubSupportTicketRepo) List(ctx context.Context, userID *int64, status *string, limit, offset int) ([]model.SupportTicket, int, error) {
	if s.listFn == nil {
		panic("listFn not configured")
	}
	return s.listFn(ctx, userID, status, limit, offset)
}

func (m *stubSupportTicketRepo) UpdateStatus(ctx context.Context, ticketID int64, status string) error {
	return nil
}

func (s *stubSupportTicketRepo) GetBookingIDByReferenceCode(ctx context.Context, ref string) (*int64, error) {
	panic("unexpected call to GetBookingIDByReferenceCode")
}

func TestSupportTicketService_ListForAdmin_NoFilter(t *testing.T) {
	repo := &stubSupportTicketRepo{
		listFn: func(ctx context.Context, userID *int64, status *string, limit, offset int) ([]model.SupportTicket, int, error) {
			if userID != nil {
				t.Fatalf("expected nil userID filter, got %v", *userID)
			}
			if status != nil {
				t.Fatalf("expected nil status filter, got %v", *status)
			}
			if limit != 50 {
				t.Fatalf("expected limit 50, got %d", limit)
			}
			if offset != 0 {
				t.Fatalf("expected offset 0, got %d", offset)
			}
			return []model.SupportTicket{{TicketID: 1}}, 1, nil
		},
	}

	svc := &SupportTicketService{repo: repo}

	resp, err := svc.ListForAdmin(context.Background(), nil, 1, 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Total != 1 {
		t.Fatalf("expected total 1, got %d", resp.Total)
	}
	if len(resp.Tickets) != 1 || resp.Tickets[0].TicketID != 1 {
		t.Fatalf("unexpected tickets result: %+v", resp.Tickets)
	}
}

func TestSupportTicketService_ListForAdmin_NormalizesStatus(t *testing.T) {
	repo := &stubSupportTicketRepo{
		listFn: func(ctx context.Context, userID *int64, status *string, limit, offset int) ([]model.SupportTicket, int, error) {
			if status == nil {
				t.Fatalf("expected status filter to be provided")
			}
			if *status != "pending" {
				t.Fatalf("expected normalized status 'pending', got %q", *status)
			}
			if limit != 50 {
				t.Fatalf("expected limit 50, got %d", limit)
			}
			if offset != 50 {
				t.Fatalf("expected offset 50, got %d", offset)
			}
			return nil, 0, nil
		},
	}

	svc := &SupportTicketService{repo: repo}
	status := " Pending "

	if _, err := svc.ListForAdmin(context.Background(), &status, 2, 50); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSupportTicketService_ListForAdmin_InvalidStatus(t *testing.T) {
	svc := &SupportTicketService{repo: &stubSupportTicketRepo{}}
	status := "unsupported"

	_, err := svc.ListForAdmin(context.Background(), &status, 1, 50)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
}

func TestSupportTicketService_ListForAdmin_RepoError(t *testing.T) {
	repoErr := errors.New("boom")
	repo := &stubSupportTicketRepo{
		listFn: func(ctx context.Context, userID *int64, status *string, limit, offset int) ([]model.SupportTicket, int, error) {
			return nil, 0, repoErr
		},
	}

	svc := &SupportTicketService{repo: repo}

	_, err := svc.ListForAdmin(context.Background(), nil, 1, 50)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, repoErr) {
		t.Fatalf("expected error to wrap repoErr, got %v", err)
	}
}
