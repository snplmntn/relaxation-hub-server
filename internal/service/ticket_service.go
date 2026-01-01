package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

type SupportTicketService struct {
	repo     repository.SupportTicketRepository
	userRepo repository.UserRepository
}

var supportTicketStatuses = map[string]struct{}{
	"pending":       {},
	"investigating": {},
	"resolved":      {},
	"closed":        {},
}

func NewSupportTicketService(repo repository.SupportTicketRepository, userRepo repository.UserRepository) *SupportTicketService {
	return &SupportTicketService{
		repo:     repo,
		userRepo: userRepo,
	}
}

// Create submits a new support ticket.
// It fetches the user's current profile info (email/phone) to store a snapshot in the ticket.
func (s *SupportTicketService) Create(ctx context.Context, userID int64, req *model.CreateSupportTicketRequest, fileURLs []string) (*model.SupportTicket, error) {
	// 1. Fetch user to get current contact info
	user, err := s.userRepo.FindUserByID(ctx, int(userID))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user: %w", err)
	}

	// 2. Determine "Connected Email/Phone" based on what we have
	connectedInfo := user.PrimaryEmail
	if connectedInfo == "" {
		connectedInfo = user.PrimaryPhone
	}

	// 3. Construct ticket model
	ticket := &model.SupportTicket{
		UserID:              &userID, // using pointer for flexibility if we allow anon later
		FullName:            user.FullName,
		ConnectedEmailPhone: connectedInfo,
		ContactEmailPhone:   req.ContactEmailPhone,
		Category:            req.Category,
		BookingID:           req.BookingID,
		Description:         req.Description,
		Status:              "pending",
	}

	// 4. Save ticket to DB
	createdTicket, err := s.repo.Create(ctx, ticket)
	if err != nil {
		return nil, fmt.Errorf("failed to create ticket: %w", err)
	}

	// 5. Handle attachments if any
	if len(fileURLs) > 0 {
		attachments := make([]model.SupportTicketAttachment, len(fileURLs))
		for i, url := range fileURLs {
			attachments[i] = model.SupportTicketAttachment{
				TicketID: createdTicket.TicketID,
				FileURL:  url,
				FileType: "image", // simplified for now
			}
		}
		if err := s.repo.CreateAttachments(ctx, attachments); err != nil {
			// In production, we might want to log this but not fail the whole request,
			// or rollback. For now, we return error.
			return nil, fmt.Errorf("failed to save attachments: %w", err)
		}
		createdTicket.Attachments = attachments
	}

	return createdTicket, nil
}

// ListForAdmin returns support tickets ordered by recency with optional status filtering.
func (s *SupportTicketService) ListForAdmin(ctx context.Context, status *string) ([]model.SupportTicket, error) {
	var normalizedStatus string
	var statusFilter *string
	if status != nil {
		trimmed := strings.TrimSpace(*status)
		if trimmed != "" {
			normalizedStatus = strings.ToLower(trimmed)
			if _, ok := supportTicketStatuses[normalizedStatus]; !ok {
				return nil, NewValidationError("invalid_status", "unsupported ticket status filter", map[string]string{"status": "allowed values: pending, investigating, resolved, closed"})
			}
			statusFilter = &normalizedStatus
		}
	}

	tickets, err := s.repo.List(ctx, statusFilter)
	if err != nil {
		return nil, fmt.Errorf("failed to list tickets: %w", err)
	}

	return tickets, nil
}
