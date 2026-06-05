// Command dumpbookings is a read-only diagnostic that prints recurring booking
// series and recent bookings (both normal and recurring-generated) with their
// scheduled times in Asia/Manila, to debug why occurrences aren't showing.
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/config"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
)

var manila = time.FixedZone("Asia/Manila", 8*60*60)

func fmtTime(t *time.Time) string {
	if t == nil {
		return "<nil>"
	}
	m := t.In(manila)
	return fmt.Sprintf("%s (%s)", m.Format("2006-01-02 15:04 MST"), m.Weekday())
}

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	pool, err := db.InitDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer pool.Close()
	ctx := context.Background()

	fmt.Printf("\n=== NOW: %s ===\n", time.Now().In(manila).Format("2006-01-02 15:04 MST (Mon)"))

	// 1. Recurring series
	fmt.Println("\n=== RECURRING SERIES (recurring_bookings) ===")
	rows, err := pool.Query(ctx, `
		SELECT recurring_id, client_id, therapist_id, service_id, status, frequency,
		       interval_value, days_of_week, day_of_month, time_of_day,
		       start_date, end_date, generated_until, created_at
		FROM recurring_bookings
		ORDER BY created_at DESC
		LIMIT 20`)
	if err != nil {
		log.Fatalf("query series: %v", err)
	}
	for rows.Next() {
		var id, clientID int64
		var therapistID, serviceID *int64
		var status, freq, timeOfDay string
		var interval int
		var daysOfWeek []int32
		var dayOfMonth *int
		var startDate time.Time
		var endDate, genUntil *time.Time
		var createdAt time.Time
		if err := rows.Scan(&id, &clientID, &therapistID, &serviceID, &status, &freq,
			&interval, &daysOfWeek, &dayOfMonth, &timeOfDay,
			&startDate, &endDate, &genUntil, &createdAt); err != nil {
			log.Fatalf("scan series: %v", err)
		}
		fmt.Printf("\n  series #%d  status=%s  freq=%s interval=%d\n", id, status, freq, interval)
		fmt.Printf("    client=%d therapist=%v service=%v\n", clientID, derefI64(therapistID), derefI64(serviceID))
		fmt.Printf("    days_of_week=%v day_of_month=%v time_of_day=%s\n", daysOfWeek, derefInt(dayOfMonth), timeOfDay)
		fmt.Printf("    start_date=%s end_date=%s\n", startDate.In(manila).Format("2006-01-02 (Mon)"), fmtTimePtrDate(endDate))
		fmt.Printf("    generated_until=%s created_at=%s\n", fmtTime(genUntil), fmtTime(&createdAt))
	}
	rows.Close()

	// 2. Recurring-generated occurrences
	fmt.Println("\n=== RECURRING OCCURRENCES (bookings WHERE recurring_id IS NOT NULL) ===")
	rows, err = pool.Query(ctx, `
		SELECT booking_id, recurring_id, client_id, therapist_id, status, scheduled_start, duration_minutes, created_at
		FROM bookings
		WHERE recurring_id IS NOT NULL
		ORDER BY scheduled_start ASC NULLS LAST
		LIMIT 60`)
	if err != nil {
		log.Fatalf("query occurrences: %v", err)
	}
	count := 0
	for rows.Next() {
		var bid, recID, clientID int64
		var therapistID *int64
		var status string
		var schedStart *time.Time
		var dur int
		var createdAt time.Time
		if err := rows.Scan(&bid, &recID, &clientID, &therapistID, &status, &schedStart, &dur, &createdAt); err != nil {
			log.Fatalf("scan occurrence: %v", err)
		}
		fmt.Printf("  bk #%d  series=%d  status=%-9s therapist=%v  start=%s  dur=%dm\n",
			bid, recID, status, derefI64(therapistID), fmtTime(schedStart), dur)
		count++
	}
	rows.Close()
	fmt.Printf("  (total occurrences shown: %d)\n", count)

	// 3. Recent normal bookings
	fmt.Println("\n=== RECENT NORMAL BOOKINGS (recurring_id IS NULL) ===")
	rows, err = pool.Query(ctx, `
		SELECT booking_id, client_id, therapist_id, status, scheduled_start, duration_minutes, created_at
		FROM bookings
		WHERE recurring_id IS NULL
		ORDER BY created_at DESC
		LIMIT 15`)
	if err != nil {
		log.Fatalf("query normal: %v", err)
	}
	for rows.Next() {
		var bid, clientID int64
		var therapistID *int64
		var status string
		var schedStart *time.Time
		var dur int
		var createdAt time.Time
		if err := rows.Scan(&bid, &clientID, &therapistID, &status, &schedStart, &dur, &createdAt); err != nil {
			log.Fatalf("scan normal: %v", err)
		}
		fmt.Printf("  bk #%d  status=%-9s therapist=%v  start=%s  created=%s\n",
			bid, status, derefI64(therapistID), fmtTime(schedStart), fmtTime(&createdAt))
	}
	rows.Close()

	fmt.Println()
}

func derefI64(p *int64) any {
	if p == nil {
		return "<nil>"
	}
	return *p
}
func derefInt(p *int) any {
	if p == nil {
		return "<nil>"
	}
	return *p
}
func fmtTimePtrDate(t *time.Time) string {
	if t == nil {
		return "<open-ended>"
	}
	return t.In(manila).Format("2006-01-02 (Mon)")
}
