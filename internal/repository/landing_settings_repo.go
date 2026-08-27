package repository

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

// LandingSettingsRepository defines persistence for the single-row landing settings.
type LandingSettingsRepository interface {
	Get(ctx context.Context) (*model.LandingSettings, error)
	Update(ctx context.Context, updates map[string]interface{}) (*model.LandingSettings, error)
}

type landingSettingsRepo struct {
	db db.DBTX
}

// NewLandingSettingsRepository creates a landing settings repository.
func NewLandingSettingsRepository(database db.DBTX) LandingSettingsRepository {
	return &landingSettingsRepo{db: database}
}

const landingSettingsSelect = `
SELECT phone_globe, phone_smart, email, address, hours,
       facebook_url, instagram_url, whatsapp_url, viber_url, application_link, updated_at
FROM landing_settings
WHERE id = 1`

func (r *landingSettingsRepo) Get(ctx context.Context) (*model.LandingSettings, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	return scanLandingSettings(r.db.QueryRow(ctx, landingSettingsSelect))
}

func (r *landingSettingsRepo) Update(ctx context.Context, updates map[string]interface{}) (*model.LandingSettings, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	if len(updates) == 0 {
		return r.Get(ctx)
	}

	// Deterministic column ordering keeps the generated SQL stable.
	cols := make([]string, 0, len(updates))
	for col := range updates {
		cols = append(cols, col)
	}
	sort.Strings(cols)

	setClauses := make([]string, 0, len(cols)+1)
	args := make([]interface{}, 0, len(cols))
	for i, col := range cols {
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", col, i+1))
		args = append(args, updates[col])
	}
	setClauses = append(setClauses, "updated_at = CURRENT_TIMESTAMP")

	query := fmt.Sprintf(`
UPDATE landing_settings
SET %s
WHERE id = 1
RETURNING phone_globe, phone_smart, email, address, hours,
          facebook_url, instagram_url, whatsapp_url, viber_url, application_link, updated_at`,
		strings.Join(setClauses, ", "))

	return scanLandingSettings(r.db.QueryRow(ctx, query, args...))
}

type rowScanner interface {
	Scan(dest ...interface{}) error
}

func scanLandingSettings(row rowScanner) (*model.LandingSettings, error) {
	var s model.LandingSettings
	if err := row.Scan(
		&s.PhoneGlobe,
		&s.PhoneSmart,
		&s.Email,
		&s.Address,
		&s.Hours,
		&s.FacebookURL,
		&s.InstagramURL,
		&s.WhatsappURL,
		&s.ViberURL,
		&s.ApplicationLink,
		&s.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &s, nil
}
