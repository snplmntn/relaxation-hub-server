package migrations_test

import (
	"os"
	"strings"
	"testing"
)

func TestRideNumericForwardMigrationWidensLegacyColumns(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read migrations directory: %v", err)
	}

	var sql strings.Builder
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		content, err := os.ReadFile(entry.Name())
		if err != nil {
			t.Fatalf("read migration %s: %v", entry.Name(), err)
		}
		sql.Write(content)
		sql.WriteByte('\n')
	}

	normalized := strings.Join(strings.Fields(strings.ToLower(sql.String())), " ")
	for _, required := range []string{
		"alter table public.rides alter column pickup_lat type double precision",
		"alter table public.rides alter column pickup_long type double precision",
		"alter table public.rides alter column dropoff_lat type double precision",
		"alter table public.rides alter column dropoff_long type double precision",
		"alter table public.rides alter column distance_km type double precision",
	} {
		if !strings.Contains(normalized, required) {
			t.Fatalf("missing ride numeric widening migration containing %q", required)
		}
	}
}

func TestRideRiderUserForwardMigrationConvertsProfileIDsInTriggers(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read migrations directory: %v", err)
	}

	var sql strings.Builder
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		content, err := os.ReadFile(entry.Name())
		if err != nil {
			t.Fatalf("read migration %s: %v", entry.Name(), err)
		}
		sql.Write(content)
		sql.WriteByte('\n')
	}

	normalized := strings.Join(strings.Fields(strings.ToLower(sql.String())), " ")
	for _, required := range []string{
		"create or replace function update_rider_performance_metrics()",
		"select user_id into v_rider_user_id from public.rider_profiles where rider_id = new.rider_id",
		"insert into public.rider_performance_metrics (rider_id) values (v_rider_user_id)",
		"where rider_id = v_rider_user_id",
		"create or replace function update_rider_wallet_on_earning()",
		"and new.rider_earnings_cents > 0",
		"insert into public.rider_wallets (rider_id, balance_cents, total_earned_cents) values (v_rider_user_id, 0, 0)",
		"insert into public.rider_transactions (rider_id, transaction_type, amount_cents, ride_id, status, description) values ( v_rider_user_id",
		"drop trigger if exists trg_update_rider_wallet on public.rides",
	} {
		if !strings.Contains(normalized, required) {
			t.Fatalf("missing rider trigger profile-to-user migration containing %q", required)
		}
	}
}

func TestRiderProfileUserForwardMigrationEnforcesSingleProfilePerUser(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read migrations directory: %v", err)
	}

	var sql strings.Builder
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		content, err := os.ReadFile(entry.Name())
		if err != nil {
			t.Fatalf("read migration %s: %v", entry.Name(), err)
		}
		sql.Write(content)
		sql.WriteByte('\n')
	}

	normalized := strings.Join(strings.Fields(strings.ToLower(sql.String())), " ")
	for _, required := range []string{
		"create temp table tmp_rider_profile_dedupe",
		"update public.rides r set rider_id = d.keep_rider_id",
		"update public.ride_offers ro set rider_id = d.keep_rider_id",
		"delete from public.rider_profiles rp using tmp_rider_profile_dedupe d",
		"create unique index if not exists idx_rider_profiles_user_id_unique on public.rider_profiles(user_id)",
	} {
		if !strings.Contains(normalized, required) {
			t.Fatalf("missing rider profile user uniqueness migration containing %q", required)
		}
	}
}

func TestRideForwardMigrationEnforcesUniqueActiveBookingLeg(t *testing.T) {
	content, err := os.ReadFile("029_enforce_unique_active_booking_rides.sql")
	if err != nil {
		t.Fatalf("read active ride uniqueness migration: %v", err)
	}

	normalized := strings.Join(strings.Fields(strings.ToLower(string(content))), " ")
	for _, required := range []string{
		"create unique index concurrently if not exists idx_rides_unique_active_booking_leg",
		"on public.rides (booking_id, (coalesce(ride_type, 'outbound')))",
		"where booking_id is not null",
		"coalesce(status, 'pending') not in ('cancelled', 'completed', 'declined', 'unmatched')",
	} {
		if !strings.Contains(normalized, required) {
			t.Fatalf("missing active ride uniqueness migration containing %q", required)
		}
	}
}
