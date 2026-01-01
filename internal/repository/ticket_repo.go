package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

type SupportTicketRepository interface {
	Create(ctx context.Context, ticket *model.SupportTicket) (*model.SupportTicket, error)
	CreateAttachments(ctx context.Context, attachments []model.SupportTicketAttachment) error
	List(ctx context.Context, status *string, limit, offset int) ([]model.SupportTicket, int, error)
	GetBookingIDByReferenceCode(ctx context.Context, ref string) (*int64, error)
}

type supportTicketRepository struct {
	db *pgxpool.Pool
}

func NewSupportTicketRepository(db *pgxpool.Pool) SupportTicketRepository {
	return &supportTicketRepository{db: db}
}

func (r *supportTicketRepository) Create(ctx context.Context, ticket *model.SupportTicket) (*model.SupportTicket, error) {
	query := `
		INSERT INTO support_tickets (
			user_id, full_name, connected_email_phone, contact_email_phone,
			category, booking_id, description, status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING ticket_id, created_at, updated_at
	`

	err := r.db.QueryRow(ctx, query,
		ticket.UserID,
		ticket.FullName,
		ticket.ConnectedEmailPhone,
		ticket.ContactEmailPhone,
		ticket.Category,
		ticket.BookingID,
		ticket.Description,
		ticket.Status,
	).Scan(&ticket.TicketID, &ticket.CreatedAt, &ticket.UpdatedAt)

	if err != nil {
		return nil, err
	}

	if ticket.BookingID != nil {
		var ref sql.NullString
		if err := r.db.QueryRow(ctx, `SELECT reference_code FROM bookings WHERE booking_id = $1`, *ticket.BookingID).Scan(&ref); err == nil {
			if ref.Valid {
				ticket.BookingReferenceCode = stringPtr(ref.String)
			}
		}
	}

	return ticket, nil
}

func (r *supportTicketRepository) CreateAttachments(ctx context.Context, attachments []model.SupportTicketAttachment) error {
	if len(attachments) == 0 {
		return nil
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	query := `
		INSERT INTO support_ticket_attachments (ticket_id, file_url, file_type)
		VALUES ($1, $2, $3)
		RETURNING attachment_id, uploaded_at
	`

	for i := range attachments {
		att := &attachments[i]
		err := tx.QueryRow(ctx, query, att.TicketID, att.FileURL, att.FileType).
			Scan(&att.AttachmentID, &att.UploadedAt)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *supportTicketRepository) List(ctx context.Context, status *string, limit, offset int) ([]model.SupportTicket, int, error) {
	countQuery := `SELECT COUNT(*) FROM support_tickets`
	countArgs := []interface{}{}
	if status != nil {
		countQuery += " WHERE status = $1"
		countArgs = append(countArgs, *status)
	}

	var total int
	if err := r.db.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	selectQuery := `
		SELECT st.ticket_id, st.user_id, st.full_name, st.connected_email_phone, st.contact_email_phone,
			st.category, st.booking_id, b.reference_code, st.description, st.status, st.created_at, st.updated_at
		FROM support_tickets st
		LEFT JOIN bookings b ON st.booking_id = b.booking_id
	`
	selectArgs := make([]interface{}, 0, len(countArgs)+2)
	selectArgs = append(selectArgs, countArgs...)
	paramIdx := len(selectArgs) + 1
	if status != nil {
		selectQuery += fmt.Sprintf(" WHERE st.status = $%d", paramIdx-1)
	}
	selectQuery += fmt.Sprintf(" ORDER BY st.created_at DESC LIMIT $%d OFFSET $%d", paramIdx, paramIdx+1)
	selectArgs = append(selectArgs, limit, offset)

	rows, err := r.db.Query(ctx, selectQuery, selectArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var tickets []model.SupportTicket
	var ticketIDs []int64
	for rows.Next() {
		var t model.SupportTicket
		var ref sql.NullString
		if err := rows.Scan(
			&t.TicketID,
			&t.UserID,
			&t.FullName,
			&t.ConnectedEmailPhone,
			&t.ContactEmailPhone,
			&t.Category,
			&t.BookingID,
			&ref,
			&t.Description,
			&t.Status,
			&t.CreatedAt,
			&t.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		if ref.Valid {
			t.BookingReferenceCode = stringPtr(ref.String)
		}
		tickets = append(tickets, t)
		ticketIDs = append(ticketIDs, t.TicketID)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	if len(ticketIDs) == 0 {
		return tickets, total, nil
	}

	placeholders := make([]string, len(ticketIDs))
	attachmentArgs := make([]interface{}, len(ticketIDs))
	for i, id := range ticketIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		attachmentArgs[i] = id
	}

	queryAttachments := fmt.Sprintf(`
		SELECT attachment_id, ticket_id, file_url, file_type, uploaded_at
		FROM support_ticket_attachments
		WHERE ticket_id IN (%s)
		ORDER BY uploaded_at ASC
	`, strings.Join(placeholders, ","))

	attRows, err := r.db.Query(ctx, queryAttachments, attachmentArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer attRows.Close()

	attachmentsByTicket := make(map[int64][]model.SupportTicketAttachment, len(ticketIDs))
	for attRows.Next() {
		var att model.SupportTicketAttachment
		if err := attRows.Scan(
			&att.AttachmentID,
			&att.TicketID,
			&att.FileURL,
			&att.FileType,
			&att.UploadedAt,
		); err != nil {
			return nil, 0, err
		}
		attachmentsByTicket[att.TicketID] = append(attachmentsByTicket[att.TicketID], att)
	}
	if err := attRows.Err(); err != nil {
		return nil, 0, err
	}

	for i := range tickets {
		if attachments, ok := attachmentsByTicket[tickets[i].TicketID]; ok {
			tickets[i].Attachments = attachments
		}
	}

	return tickets, total, nil
}

func stringPtr(s string) *string {
	return &s
}

func (r *supportTicketRepository) GetBookingIDByReferenceCode(ctx context.Context, ref string) (*int64, error) {
	var id int64
	err := r.db.QueryRow(ctx, `SELECT booking_id FROM bookings WHERE reference_code = $1`, ref).Scan(&id)
	if err != nil {
		return nil, err
	}
	return &id, nil
}
