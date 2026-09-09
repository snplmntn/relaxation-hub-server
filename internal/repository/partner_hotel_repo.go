package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

type PartnerHotelRepository interface {
	GetHotelAccess(ctx context.Context, userID int64) (*model.HotelAccess, error)
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
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var active bool
	if err := tx.QueryRow(ctx, `SELECT is_active FROM partner_hotels WHERE partner_hotel_id=$1 FOR SHARE`, staff.PartnerHotelID).Scan(&active); err != nil {
		return err
	}
	if !active {
		return fmt.Errorf("cannot add staff to an inactive partnered hotel")
	}
	if err := saveHotelAccount(ctx, tx, staff); err != nil {
		return hotelAccountError(err)
	}
	err = tx.QueryRow(ctx, `
		INSERT INTO partner_hotel_staff
			(partner_hotel_id, full_name, position, email, phone, is_active, user_id, access_role)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING partner_hotel_staff_id, created_at, updated_at`,
		staff.PartnerHotelID, staff.FullName, staff.Position, staff.Email, staff.Phone, staff.IsActive, staff.UserID, staff.AccessRole,
	).Scan(&staff.PartnerHotelStaffID, &staff.CreatedAt, &staff.UpdatedAt)
	if err != nil {
		return hotelAccountError(err)
	}
	return tx.Commit(ctx)
}

func (r *partnerHotelRepo) GetStaff(ctx context.Context, hotelID, staffID int64) (*model.PartnerHotelStaff, error) {
	var staff model.PartnerHotelStaff
	err := r.db.QueryRow(ctx, `
		SELECT partner_hotel_staff_id, partner_hotel_id, full_name, position,
			email, phone, is_active, created_at, updated_at, user_id, access_role
		FROM partner_hotel_staff
		WHERE partner_hotel_id = $1 AND partner_hotel_staff_id = $2`, hotelID, staffID).Scan(
		&staff.PartnerHotelStaffID, &staff.PartnerHotelID, &staff.FullName, &staff.Position,
		&staff.Email, &staff.Phone, &staff.IsActive, &staff.CreatedAt, &staff.UpdatedAt, &staff.UserID, &staff.AccessRole,
	)
	if err != nil {
		return nil, err
	}
	return &staff, nil
}

func (r *partnerHotelRepo) ListStaff(ctx context.Context, hotelID int64) ([]model.PartnerHotelStaff, error) {
	rows, err := r.db.Query(ctx, `
		SELECT partner_hotel_staff_id, partner_hotel_id, full_name, position,
			email, phone, is_active, created_at, updated_at, user_id, access_role
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
			&staff.Email, &staff.Phone, &staff.IsActive, &staff.CreatedAt, &staff.UpdatedAt, &staff.UserID, &staff.AccessRole,
		); err != nil {
			return nil, err
		}
		staffMembers = append(staffMembers, staff)
	}
	return staffMembers, rows.Err()
}

func (r *partnerHotelRepo) UpdateStaff(ctx context.Context, staff *model.PartnerHotelStaff) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// Lock membership before provisioning to prevent two accounts for the same contact.
	if err := tx.QueryRow(ctx, `SELECT user_id FROM partner_hotel_staff WHERE partner_hotel_id=$1 AND partner_hotel_staff_id=$2 FOR UPDATE`, staff.PartnerHotelID, staff.PartnerHotelStaffID).Scan(&staff.UserID); err != nil {
		return err
	}
	if staff.UserID == nil && staff.PasswordHash != "" {
		var active bool
		if err := tx.QueryRow(ctx, `SELECT is_active FROM partner_hotels WHERE partner_hotel_id=$1 FOR SHARE`, staff.PartnerHotelID).Scan(&active); err != nil {
			return err
		}
		if !active {
			return fmt.Errorf("cannot enable access for an inactive partnered hotel")
		}
	}
	if err := saveHotelAccount(ctx, tx, staff); err != nil {
		return hotelAccountError(err)
	}
	command, err := tx.Exec(ctx, `
		UPDATE partner_hotel_staff
		SET full_name = $3, position = $4, email = $5, phone = $6,
			is_active = $7, user_id = $8, access_role = $9, updated_at = CURRENT_TIMESTAMP
		WHERE partner_hotel_id = $1 AND partner_hotel_staff_id = $2`,
		staff.PartnerHotelID, staff.PartnerHotelStaffID, staff.FullName,
		staff.Position, staff.Email, staff.Phone, staff.IsActive, staff.UserID, staff.AccessRole,
	)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return tx.Commit(ctx)
}

func saveHotelAccount(ctx context.Context, tx pgx.Tx, staff *model.PartnerHotelStaff) error {
	if staff.UserID == nil {
		if staff.PasswordHash == "" {
			return nil
		}
		var id int64
		if err := tx.QueryRow(ctx, `INSERT INTO users (full_name, role, primary_email, primary_phone, account_status)
			VALUES ($1,$2,$3,$4,CASE WHEN $5 THEN 'active' ELSE 'inactive' END) RETURNING user_id`,
			staff.FullName, staff.AccessRole, staff.Email, staff.Phone, staff.IsActive).Scan(&id); err != nil {
			return err
		}
		staff.UserID = &id
		_, err := tx.Exec(ctx, `INSERT INTO user_auth_identities (user_id,provider,provider_key,password_hash) VALUES ($1,'email',$2,$3)`, id, staff.Email, staff.PasswordHash)
		return err
	}
	command, err := tx.Exec(ctx, `UPDATE users SET full_name=$2, role=$3, primary_email=$4, primary_phone=$5,
		account_status=CASE WHEN account_status IN ('active','inactive') THEN CASE WHEN $6 THEN 'active' ELSE 'inactive' END ELSE account_status END,
		updated_at=NOW() WHERE user_id=$1 AND role IN ('hotel_admin','hotel_staff')`,
		*staff.UserID, staff.FullName, staff.AccessRole, staff.Email, staff.Phone, staff.IsActive)
	if err != nil {
		return err
	}
	if command.RowsAffected() != 1 {
		return fmt.Errorf("hotel account is unavailable")
	}
	_, err = tx.Exec(ctx, `UPDATE user_auth_identities SET provider_key=$2, password_hash=CASE WHEN $3='' THEN password_hash ELSE $3 END WHERE user_id=$1 AND provider='email'`, *staff.UserID, staff.Email, staff.PasswordHash)
	return err
}

func hotelAccountError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		if pgErr.ConstraintName == "idx_users_primary_phone_unique" {
			return fmt.Errorf("phone number is already in use; use a different staff phone number or leave it blank")
		}
		return fmt.Errorf("email is already in use; choose a separate email for this hotel account")
	}
	return err
}

func (r *partnerHotelRepo) GetHotelAccess(ctx context.Context, userID int64) (*model.HotelAccess, error) {
	var access model.HotelAccess
	err := r.db.QueryRow(ctx, `SELECT h.partner_hotel_id,h.hotel_name,h.city,s.partner_hotel_staff_id,s.full_name,s.access_role
		FROM partner_hotel_staff s JOIN partner_hotels h USING (partner_hotel_id) JOIN users u ON u.user_id=s.user_id
		WHERE s.user_id=$1 AND s.is_active AND h.is_active AND u.account_status='active'
		AND u.deleted_at IS NULL AND u.role=s.access_role`, userID).Scan(
		&access.PartnerHotelID, &access.HotelName, &access.City, &access.StaffID, &access.FullName, &access.AccessRole)
	if err != nil {
		return nil, err
	}
	return &access, nil
}
