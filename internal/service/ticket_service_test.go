package service

import (
	"context"
	"errors"
	"testing"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

type stubSupportTicketRepo struct {
	listFn func(ctx context.Context, status *string) ([]model.SupportTicket, error)
}

func (s *stubSupportTicketRepo) Create(ctx context.Context, ticket *model.SupportTicket) (*model.SupportTicket, error) {
	panic("unexpected call to Create")
}

func (s *stubSupportTicketRepo) CreateAttachments(ctx context.Context, attachments []model.SupportTicketAttachment) error {
	panic("unexpected call to CreateAttachments")
}

func (s *stubSupportTicketRepo) List(ctx context.Context, status *string) ([]model.SupportTicket, error) {
	if s.listFn == nil {
		panic("listFn not configured")
	}
	return s.listFn(ctx, status)
}

func TestSupportTicketService_ListForAdmin_NoFilter(t *testing.T) {
	repo := &stubSupportTicketRepo{
		listFn: func(ctx context.Context, status *string) ([]model.SupportTicket, error) {
			if status != nil {
				t.Fatalf("expected nil status filter, got %v", *status)
			}
			return []model.SupportTicket{{TicketID: 1}}, nil
		},
	}

	svc := &SupportTicketService{repo: repo}

	tickets, err := svc.ListForAdmin(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tickets) != 1 || tickets[0].TicketID != 1 {
		t.Fatalf("unexpected tickets result: %+v", tickets)
	}
}

func TestSupportTicketService_ListForAdmin_NormalizesStatus(t *testing.T) {
	repo := &stubSupportTicketRepo{
		listFn: func(ctx context.Context, status *string) ([]model.SupportTicket, error) {
			if status == nil {
				t.Fatalf("expected status filter to be provided")
			}
			if *status != "pending" {
				t.Fatalf("expected normalized status 'pending', got %q", *status)
			}
			return nil, nil
		},
	}

	svc := &SupportTicketService{repo: repo}
	status := " Pending "

	if _, err := svc.ListForAdmin(context.Background(), &status); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSupportTicketService_ListForAdmin_InvalidStatus(t *testing.T) {
	svc := &SupportTicketService{repo: &stubSupportTicketRepo{}}
	status := "unsupported"

	_, err := svc.ListForAdmin(context.Background(), &status)
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
		listFn: func(ctx context.Context, status *string) ([]model.SupportTicket, error) {
			return nil, repoErr
		},
	}

	svc := &SupportTicketService{repo: repo}

	_, err := svc.ListForAdmin(context.Background(), nil)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, repoErr) {
		t.Fatalf("expected error to wrap repoErr, got %v", err)
	}
}
