package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/snplmntn/relaxation-hub-server/internal/config"
	"github.com/snplmntn/relaxation-hub-server/internal/handler"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

// MockStorageService implements service.StorageService for testing
type MockStorageService struct {
	Files map[string][]byte
}

func NewMockStorageService() *MockStorageService {
	return &MockStorageService{
		Files: make(map[string][]byte),
	}
}

func (m *MockStorageService) UploadFile(ctx context.Context, key string, body io.Reader, contentType string) (string, error) {
	data, err := io.ReadAll(body)
	if err != nil {
		return "", err
	}
	m.Files[key] = data
	return "https://mock-storage.com/" + key, nil
}

func (m *MockStorageService) GetFileURL(key string) string {
	return "https://mock-storage.com/" + key
}

func (m *MockStorageService) GetPresignedURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	return "https://mock-storage.com/" + key + "?token=presigned", nil
}

func (m *MockStorageService) DeleteFile(ctx context.Context, key string) error {
	delete(m.Files, key)
	return nil
}

func (m *MockStorageService) GenerateKey(prefix, filename string) string {
	return fmt.Sprintf("%s/%s", prefix, filename)
}

func (m *MockStorageService) IsConfigured() bool {
	return true
}

func SetupBookingRouterWithMockStorage(pool *pgxpool.Pool, cfg *config.Config, storage service.StorageService) *chi.Mux {
	r := chi.NewRouter()

	bookingRepo := repository.NewBookingRepository(pool)
	promotionRepo := repository.NewPromotionRepository(pool)
	paymentRepo := repository.NewPaymentRepository(pool)

	// Initialize other required repositories
	queueRepo := repository.NewAssignmentQueueRepository(pool)
	therapistRepo := repository.NewTherapistRepository(pool)
	offerRepo := repository.NewBookingOfferRepository(pool)
	serviceRepo := repository.NewServiceRepository(pool)
	addressRepo := repository.NewAddressRepository(pool)
	userRepo := repository.NewUserRepository(pool)
	extRepo := repository.NewExtensionRequestRepository(pool)

	bookingService := service.NewBookingService(bookingRepo, promotionRepo, pool, queueRepo, therapistRepo, offerRepo, serviceRepo, addressRepo, userRepo, nil, nil, extRepo, nil, nil)
	paymentService := service.NewPaymentService(paymentRepo)

	// In BookingHandler, we pass storageService
	bookingHandler := handler.NewBookingHandler(bookingService, paymentService, serviceRepo, addressRepo, therapistRepo, storage)

	r.Route("/api/v1", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(func(next http.Handler) http.Handler {
				return middleware.AuthMiddleware(next, cfg.JWTKey)
			})

			r.Route("/bookings", func(r chi.Router) {
				r.Post("/", bookingHandler.CreateBooking)
				r.Post("/{id}/payment-proof", bookingHandler.UploadPaymentProof)
				r.Delete("/{id}/payment-proof", bookingHandler.CancelPaymentProof)
			})
		})
	})

	return r
}

func TestIntegration_PaymentProof_UploadAndCancel(t *testing.T) {
	pool := SetupTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer CleanupTestDB(t, pool)

	mockStorage := NewMockStorageService()
	router := SetupBookingRouterWithMockStorage(pool, getTestConfig(), mockStorage)
	ctx := context.Background()

	// 1. Setup Data
	// Create Client
	clientEmail := fmt.Sprintf("client_proof_%d@test.com", time.Now().UnixNano())
	clientToken, _, _ := createTestUser(t, pool, clientEmail, "client")

	// Create Service
	serviceID := createTestService(t, pool)

	// Create Address using shared helper, requires passing a router that handles /api/v1/addresses
	// Make a temporary router for address creation since SetupBookingRouterWithMockStorage doesn't declare it (or reuse logic)
	// Actually, SetupBookingRouterWithMockStorage ONLY has bookings route.
	// We should probably just insert address directly for simplicity or update the router.
	// We'll update SetupBookingRouterWithMockStorage to include addresses? Or just use direct SQL as before?
	// The original local test used direct SQL for address. Let's stick to that for simplicity to avoid router complexity,
	// OR reuse createTestAddress by creating a full router.
	// Wait, createTestUser uses its own router internally. createTestAddress takes a router.
	// Let's just do direct SQL for address to avoid dependency hell in this specific test setup.
	var addressID int64
	err := pool.QueryRow(ctx, `INSERT INTO addresses (user_id, street_address, city, is_default) VALUES ((SELECT user_id FROM users WHERE primary_email=$1), '123 St', 'City', true) RETURNING address_id`, clientEmail).Scan(&addressID)
	if err != nil {
		t.Fatalf("failed to create address: %v", err)
	}

	// Create Booking
	bookingBody := map[string]interface{}{
		"service_id":          serviceID,
		"address_id":          addressID,
		"scheduled_start":     time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		"duration_minutes":    60,
		"payment_method":      "gcash",
		"raw_total":           1500,
		"gender_preference":   "any",
		"pressure_preference": "medium",
	}
	body, _ := json.Marshal(bookingBody)
	req := httptest.NewRequest("POST", "/api/v1/bookings", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+clientToken)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("Create booking failed: %d %s", rr.Code, rr.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	bookingID := int64(resp["booking_id"].(float64))

	// 2. Upload Proof
	t.Logf("Uploading proof for booking %d...", bookingID)

	uploadProof := func(token string) {
		fileBody := new(bytes.Buffer)
		writer := multipart.NewWriter(fileBody)
		part, err := writer.CreateFormFile("proof_file", "receipt.jpg")
		if err != nil {
			t.Fatal(err)
		}
		part.Write([]byte("fake image data"))
		writer.Close()

		req = httptest.NewRequest("POST", fmt.Sprintf("/api/v1/bookings/%d/payment-proof", bookingID), fileBody)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+token)

		rr = httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("Upload proof failed: %d %s", rr.Code, rr.Body.String())
		}
	}
	uploadProof(clientToken)
	t.Log("✓ Client Upload success")

	// Verify DB status
	checkProof := func() string {
		var proofURL *string
		err = pool.QueryRow(ctx, "SELECT proof_url FROM payments WHERE booking_id=$1", bookingID).Scan(&proofURL)
		if err != nil {
			t.Fatalf("Failed to query payment: %v", err)
		}
		if proofURL == nil {
			return ""
		}
		return *proofURL
	}
	if checkProof() == "" {
		t.Error("Expected proof_url to be set")
	}

	// 3. Client Cancel Proof
	t.Log("Cancelling proof as client...")
	req = httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/bookings/%d/payment-proof", bookingID), nil)
	req.Header.Set("Authorization", "Bearer "+clientToken)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Cancel proof failed: %d %s", rr.Code, rr.Body.String())
	}
	t.Log("✓ Client Cancel success")
	if checkProof() != "" {
		t.Error("Expected proof_url to be NULL")
	}

	// 4. Test Therapist Cancellation
	// Re-upload proof
	uploadProof(clientToken)

	// Create Therapist
	therapistEmail := fmt.Sprintf("therapist_proof_%d@test.com", time.Now().UnixNano())
	therapistToken, therapistID, _ := createTestUser(t, pool, therapistEmail, "therapist")

	// Create another therapist (unassigned)
	otherTherapistEmail := fmt.Sprintf("other_therapist_proof_%d@test.com", time.Now().UnixNano())
	otherTherapistToken, _, _ := createTestUser(t, pool, otherTherapistEmail, "therapist")

	// Create Therapist Profile to be safe
	_, err = pool.Exec(ctx, "INSERT INTO therapist_profiles (therapist_id, accept_assignments) VALUES ($1, true) ON CONFLICT DO NOTHING", therapistID)
	if err != nil {
		t.Fatalf("Failed to create therapist profile: %v", err)
	}

	// Assign Therapist to Booking
	cmd, err := pool.Exec(ctx, "UPDATE bookings SET therapist_id=$1, status='assigned' WHERE booking_id=$2", therapistID, bookingID)
	if err != nil {
		t.Fatalf("Failed to assign therapist: %v", err)
	}
	if cmd.RowsAffected() == 0 {
		t.Fatal("Failed to assign therapist: no rows updated")
	}

	// Try cancel with unassigned therapist
	t.Log("Attempting cancel with unassigned therapist...")
	req = httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/bookings/%d/payment-proof", bookingID), nil)
	req.Header.Set("Authorization", "Bearer "+otherTherapistToken)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for unassigned therapist, got %d", rr.Code)
	}

	// Try cancel with assigned therapist
	t.Log("Cancelling proof as assigned therapist...")
	req = httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/bookings/%d/payment-proof", bookingID), nil)
	req.Header.Set("Authorization", "Bearer "+therapistToken)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Cancel proof (therapist) failed: %d %s", rr.Code, rr.Body.String())
	}
	t.Log("✓ Therapist Cancel success")
	if checkProof() != "" {
		t.Error("Expected proof_url to be NULL")
	}
}
