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

func (s *SupportTicketService) GetBookingIDByReferenceCode(ctx context.Context, ref string) (*int64, error) {
	return s.repo.GetBookingIDByReferenceCode(ctx, ref)
}

// Create submits a new support ticket.
// It fetches the user's current profile info (email/phone) to store a snapshot in the ticket.
func (s *SupportTicketService) Create(ctx context.Context, userID int64, req *model.CreateSupportTicketRequest, fileURLs []string) (*model.SupportTicket, error) {
	var user *model.User
	var err error

	// 1. Fetch user (if authenticated)
	if userID != 0 {
		user, err = s.userRepo.FindUserByID(ctx, int(userID))
		if err != nil {
			return nil, fmt.Errorf("failed to fetch user: %w", err)
		}
	}

	// 2. Determine "Connected Email/Phone" based on what we have
	connectedInfo := ""
	fullName := "Guest"

	if user != nil {
		connectedInfo = user.PrimaryEmail
		if connectedInfo == "" {
			connectedInfo = user.PrimaryPhone
		}
		fullName = user.FullName
	} else {
		// Anonymous: use contact info from request as connected info for reference
		connectedInfo = req.ContactEmailPhone
	}

	// 3. Construct ticket model
	var userIDPtr *int64
	if userID != 0 {
		userIDPtr = &userID
	}

	ticket := &model.SupportTicket{
		UserID:              userIDPtr,
		FullName:            fullName,
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
func (s *SupportTicketService) ListForAdmin(ctx context.Context, status *string, page, limit int) (*model.PaginatedSupportTicketsResponse, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit
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

	tickets, total, err := s.repo.List(ctx, nil, statusFilter, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list tickets: %w", err)
	}

	totalPages := 0
	if total > 0 {
		totalPages = (total + limit - 1) / limit
	}
	hasMore := page < totalPages

	return &model.PaginatedSupportTicketsResponse{
		Tickets:    tickets,
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
		HasMore:    hasMore,
	}, nil
}

func (s *SupportTicketService) UpdateStatus(ctx context.Context, ticketID int64, status string) error {
	normalizedStatus := strings.ToLower(strings.TrimSpace(status))
	if _, ok := supportTicketStatuses[normalizedStatus]; !ok {
		return NewValidationError("invalid_status", "unsupported ticket status", map[string]string{"status": "allowed using: pending, investigating, resolved, closed"})
	}

	if err := s.repo.UpdateStatus(ctx, ticketID, normalizedStatus); err != nil {
		return fmt.Errorf("failed to update ticket status: %w", err)
	}

	return nil
}

// ListForUser returns the authenticated user's own support tickets.
func (s *SupportTicketService) ListForUser(ctx context.Context, userID int64, page, limit int) (*model.PaginatedSupportTicketsResponse, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit

	tickets, total, err := s.repo.List(ctx, &userID, nil, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list tickets: %w", err)
	}

	totalPages := 0
	if total > 0 {
		totalPages = (total + limit - 1) / limit
	}
	hasMore := page < totalPages

	return &model.PaginatedSupportTicketsResponse{
		Tickets:    tickets,
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
		HasMore:    hasMore,
	}, nil
}

// ListForUser returns the authenticated user's own support tickets.
