package handler

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

const bookingReportExportBatchSize = 200

func (h *BookingHandler) ExportBookingReportWorkbook(w http.ResponseWriter, r *http.Request) {
	clientID, err := parseOptionalPositiveInt64(r.URL.Query().Get("client_id"), "client_id")
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	rows, err := h.collectBookingReportRows(r.Context(), clientID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	now := time.Now()
	workbook, err := service.BuildBookingReportWorkbook(rows, service.BookingReportScope{
		ClientID:    clientID,
		GeneratedAt: now,
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeWorkbook(w, bookingReportFilename(clientID, now), workbook)
}

func (h *BookingHandler) collectBookingReportRows(ctx context.Context, clientID *int64) ([]repository.BookingDetailsResult, error) {
	rows := make([]repository.BookingDetailsResult, 0)
	offset := 0
	total := 0

	for {
		var batch []repository.BookingDetailsResult
		var err error
		if clientID != nil {
			batch, total, err = h.bookingService.ListByClientWithDetailsPaginated(ctx, *clientID, bookingReportExportBatchSize, offset)
		} else {
			batch, total, err = h.bookingService.ListAllWithDetailsPaginated(ctx, bookingReportExportBatchSize, offset, "", "", "", "")
		}
		if err != nil {
			return nil, err
		}
		rows = append(rows, batch...)
		if len(batch) == 0 || len(rows) >= total {
			break
		}
		offset += bookingReportExportBatchSize
	}

	return rows, nil
}

func parseOptionalPositiveInt64(value, field string) (*int64, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return nil, fmt.Errorf("%s must be a positive integer", field)
	}
	return &parsed, nil
}

func bookingReportFilename(clientID *int64, now time.Time) string {
	timestamp := now.Format("20060102-150405")
	if clientID != nil {
		return fmt.Sprintf("booking-report-client-%d-%s.xlsx", *clientID, timestamp)
	}
	return fmt.Sprintf("booking-report-all-clients-%s.xlsx", timestamp)
}
