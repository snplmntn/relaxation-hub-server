package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

type PartnerHotelRepository interface {
	CreateHotel(ctx context.Context, hotel *model.PartnerHotel) error
	GetHotel(ctx context.Context, hotelID int64) (*model.PartnerHotel, error)
	ListHotels(ctx context.Context) ([]model.PartnerHotel, error)
	UpdateHotel(ctx context.Context, hotel *model.PartnerHotel) error
	CreateStaff(ctx context.Context, staff *model.PartnerHotelStaff) error
	GetStaff(ctx context.Context, hotelID, staffID int64) (*model.PartnerHotelStaff, error)
	ListStaff(ctx context.Context, hotelID int64) ([]model.PartnerHotelStaff, error)
	UpdateStaff(ctx context.Context, staff *model.PartnerHotelStaff) error
}

type partnerHotelRepo struct {
	db db.DBTX
}

func NewPartnerHotelRepository(database db.DBTX) PartnerHotelRepository {
	return &partnerHotelRepo{db: database}
}

func (r *partnerHotelRepo) CreateHotel(ctx context.Context, hotel *model.PartnerHotel) error {
	return r.db.QueryRow(ctx, `
		INSERT INTO partner_hotels
			(hotel_name, address_line, city, contact_person, email, phone, notes, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING partner_hotel_id, created_at, updated_at`,
		hotel.HotelName, hotel.AddressLine, hotel.City, hotel.ContactPerson,
		hotel.Email, hotel.Phone, hotel.Notes, hotel.IsActive,
	).Scan(&hotel.PartnerHotelID, &hotel.CreatedAt, &hotel.UpdatedAt)
}

func (r *partnerHotelRepo) GetHotel(ctx context.Context, hotelID int64) (*model.PartnerHotel, error) {
	var hotel model.PartnerHotel
	err := r.db.QueryRow(ctx, `
		SELECT h.partner_hotel_id, h.hotel_name, h.address_line, h.city,
			h.contact_person, h.email, h.phone, h.notes, h.is_active,
			COUNT(s.partner_hotel_staff_id) FILTER (WHERE s.is_active),
			h.created_at, h.updated_at
		FROM partner_hotels h
		LEFT JOIN partner_hotel_staff s ON s.partner_hotel_id = h.partner_hotel_id
		WHERE h.partner_hotel_id = $1
		GROUP BY h.partner_hotel_id`, hotelID).Scan(
		&hotel.PartnerHotelID, &hotel.HotelName, &hotel.AddressLine, &hotel.City,
		&hotel.ContactPerson, &hotel.Email, &hotel.Phone, &hotel.Notes, &hotel.IsActive,
		&hotel.StaffCount, &hotel.CreatedAt, &hotel.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &hotel, nil
}

func (r *partnerHotelRepo) ListHotels(ctx context.Context) ([]model.PartnerHotel, error) {
	rows, err := r.db.Query(ctx, `
		SELECT h.partner_hotel_id, h.hotel_name, h.address_line, h.city,
			h.contact_person, h.email, h.phone, h.notes, h.is_active,
			COUNT(s.partner_hotel_staff_id) FILTER (WHERE s.is_active),
			h.created_at, h.updated_at
		FROM partner_hotels h
		LEFT JOIN partner_hotel_staff s ON s.partner_hotel_id = h.partner_hotel_id
		GROUP BY h.partner_hotel_id
		ORDER BY h.is_active DESC, lower(h.hotel_name), h.partner_hotel_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	hotels := make([]model.PartnerHotel, 0)
	for rows.Next() {
		var hotel model.PartnerHotel
		if err := rows.Scan(
			&hotel.PartnerHotelID, &hotel.HotelName, &hotel.AddressLine, &hotel.City,
			&hotel.ContactPerson, &hotel.Email, &hotel.Phone, &hotel.Notes, &hotel.IsActive,
			&hotel.StaffCount, &hotel.CreatedAt, &hotel.UpdatedAt,
		); err != nil {
			return nil, err
		}
		hotels = append(hotels, hotel)
	}
	return hotels, rows.Err()
}

func (r *partnerHotelRepo) UpdateHotel(ctx context.Context, hotel *model.PartnerHotel) error {
	command, err := r.db.Exec(ctx, `
		UPDATE partner_hotels
		SET hotel_name = $2, address_line = $3, city = $4, contact_person = $5,
			email = $6, phone = $7, notes = $8, is_active = $9,
			updated_at = CURRENT_TIMESTAMP
		WHERE partner_hotel_id = $1`,
		hotel.PartnerHotelID, hotel.HotelName, hotel.AddressLine, hotel.City,
		hotel.ContactPerson, hotel.Email, hotel.Phone, hotel.Notes, hotel.IsActive,
	)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *partnerHotelRepo) CreateStaff(ctx context.Context, staff *model.PartnerHotelStaff) error {
	return r.db.QueryRow(ctx, `
		INSERT INTO partner_hotel_staff
			(partner_hotel_id, full_name, position, email, phone, is_active)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING partner_hotel_staff_id, created_at, updated_at`,
		staff.PartnerHotelID, staff.FullName, staff.Position, staff.Email, staff.Phone, staff.IsActive,
	).Scan(&staff.PartnerHotelStaffID, &staff.CreatedAt, &staff.UpdatedAt)
}

func (r *partnerHotelRepo) GetStaff(ctx context.Context, hotelID, staffID int64) (*model.PartnerHotelStaff, error) {
	var staff model.PartnerHotelStaff
	err := r.db.QueryRow(ctx, `
		SELECT partner_hotel_staff_id, partner_hotel_id, full_name, position,
			email, phone, is_active, created_at, updated_at
		FROM partner_hotel_staff
		WHERE partner_hotel_id = $1 AND partner_hotel_staff_id = $2`, hotelID, staffID).Scan(
		&staff.PartnerHotelStaffID, &staff.PartnerHotelID, &staff.FullName, &staff.Position,
		&staff.Email, &staff.Phone, &staff.IsActive, &staff.CreatedAt, &staff.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &staff, nil
}

func (r *partnerHotelRepo) ListStaff(ctx context.Context, hotelID int64) ([]model.PartnerHotelStaff, error) {
	rows, err := r.db.Query(ctx, `
		SELECT partner_hotel_staff_id, partner_hotel_id, full_name, position,
			email, phone, is_active, created_at, updated_at
		FROM partner_hotel_staff
		WHERE partner_hotel_id = $1
		ORDER BY is_active DESC, lower(full_name), partner_hotel_staff_id`, hotelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	staffMembers := make([]model.PartnerHotelStaff, 0)
	for rows.Next() {
		var staff model.PartnerHotelStaff
		if err := rows.Scan(
			&staff.PartnerHotelStaffID, &staff.PartnerHotelID, &staff.FullName, &staff.Position,
			&staff.Email, &staff.Phone, &staff.IsActive, &staff.CreatedAt, &staff.UpdatedAt,
		); err != nil {
			return nil, err
		}
		staffMembers = append(staffMembers, staff)
	}
	return staffMembers, rows.Err()
}

func (r *partnerHotelRepo) UpdateStaff(ctx context.Context, staff *model.PartnerHotelStaff) error {
	command, err := r.db.Exec(ctx, `
		UPDATE partner_hotel_staff
		SET full_name = $3, position = $4, email = $5, phone = $6,
			is_active = $7, updated_at = CURRENT_TIMESTAMP
		WHERE partner_hotel_id = $1 AND partner_hotel_staff_id = $2`,
		staff.PartnerHotelID, staff.PartnerHotelStaffID, staff.FullName,
		staff.Position, staff.Email, staff.Phone, staff.IsActive,
	)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
