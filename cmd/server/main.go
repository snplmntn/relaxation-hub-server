package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	cors "github.com/go-chi/cors"
	"github.com/snplmntn/relaxation-hub-server/internal/broadcaster"
	internalConfig "github.com/snplmntn/relaxation-hub-server/internal/config"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/handler"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/oauth"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
	ws "github.com/snplmntn/relaxation-hub-server/internal/websocket"
)

// headResponseWriter is a thin wrapper that prevents body bytes from being
// actually written. It still allows the handler to set headers/status codes
// so we can reuse the GET handler for HEAD probes.
type headResponseWriter struct {
	http.ResponseWriter
}

func (h *headResponseWriter) Write(b []byte) (int, error) {
	// Pretend we wrote the bytes but discard them.
	return len(b), nil
}

func main() {


	cfg, err := internalConfig.LoadConfig()
	if err != nil {
		log.Fatal("Error loading .env file: ", err)
	}
	pool, err := db.InitDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v\n", err)
	}
	defer db.CloseDB(pool)

	// Initialize Storage Service (S3)
	storageService := service.NewS3StorageService(context.Background(), service.S3Config{
		Bucket: cfg.AWSS3Bucket,
		Region: cfg.AWSRegion,
	})

	// Initialize legacy WebSocket hub (gorilla) for existing clients
	hub := ws.NewHub()
	hub.SetPool(pool) // Allow hub to perform user enrichment queries
	go hub.Run()

	// Initialize structured logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)
	slog.Info("starting relaxation-hub server")

	// Wire the hub into the broadcaster adapter so BroadcastToUser calls work
	broadcaster.SetHub(hub)
	
	// Mapbox Geocoding Service (SOTA 2026)
	mapboxToken := os.Getenv("MAPBOX_API_TOKEN")
	
	realGeocoder := service.NewMapboxGeocoder(mapboxToken)
	geocoder, err := service.NewCachedGeocoder(realGeocoder, 1000, 24*time.Hour)
	if err != nil {
		slog.Error("failed to create cached geocoder", "error", err)
		geocoder = realGeocoder // Fallback to non-cached
	}

	// Update services that require geocoding
	// addressService.SetGeocoder(geocoder) // Moved below

	// --- Background Workers Context ---
	// Create a cancelable context for workers and rate limiters
	// This context is shared by all background goroutines for graceful shutdown
	workerCtx, cancelWorkers := context.WithCancel(context.Background())
	var workerGroup sync.WaitGroup

	// Create the chi router
	r := chi.NewRouter()

	// Wire dependencies
	userRepo := repository.NewUserRepository(pool)
	authService := service.NewAuthService(userRepo, cfg)
	rateLimiter := middleware.NewRateLimiter(workerCtx, pool, middleware.DefaultRateLimitConfig())
	ticketLimiter := middleware.NewRateLimiter(workerCtx, pool, middleware.RateLimitConfig{
		MaxAttempts:     2,
		LockoutDuration: 10 * time.Minute,
		ResetWindow:     10 * time.Minute,
		CheckInterval:   1 * time.Minute,
	})
	referralRepo := repository.NewReferralRepository(pool)
	referralService := service.NewReferralService(referralRepo)
	authHandler := handler.NewAuthHandler(authService, rateLimiter, referralService)
	addressRepo := repository.NewAddressRepository(pool)
	addressService := service.NewAddressService(addressRepo, nil)
	addressService.SetGeocoder(geocoder)
	addressHandler := handler.NewAddressHandler(addressService)
	bookingRepo := repository.NewBookingRepository(pool)
	therapistRepo := repository.NewTherapistRepository(pool)
	promotionRepo := repository.NewPromotionRepository(pool)
	assignmentQueueRepo := repository.NewAssignmentQueueRepository(pool)
	offerRepo := repository.NewBookingOfferRepository(pool)
	serviceRepo := repository.NewServiceRepository(pool)
	ticketRepo := repository.NewSupportTicketRepository(pool)

	// Initialize FCM service for push notifications
	fcmService, err := service.NewFCMService(context.Background())
	if err != nil {
		log.Printf("Warning: FCM service initialization failed: %v (push notifications will be disabled)", err)
	}

	notificationRepo := repository.NewNotificationRepository(pool)
	notificationService := service.NewNotificationService(notificationRepo, userRepo, fcmService)
	notificationHandler := handler.NewNotificationHandler(notificationService)

	messageRepo := repository.NewMessageRepository(pool)
	messageService := service.NewMessageService(messageRepo, notificationService, userRepo, hub)

	walletRepo := repository.NewWalletRepository(pool)
	walletService := service.NewWalletService(pool, walletRepo, bookingRepo)
	walletHandler := handler.NewWalletHandler(walletService)

	// Ride Module
	rideRepo := repository.NewRideRepository(pool)
	rideOfferRepo := repository.NewRideOfferRepository(pool)
	ridePricingService := service.NewRidePricingService(pool)
	rideMatchingService := service.NewRideMatchingService(pool)
	rideService := service.NewRideService(rideRepo, rideOfferRepo, ridePricingService, rideMatchingService, pool)
	rideService.SetNotificationService(notificationService)
	rideService.SetMessageService(messageService)
	rideService.SetGeocoder(geocoder)
	rideHandler := handler.NewRideHandler(rideService)
	riderHandler := handler.NewRiderHandler(rideService)
	adminPricingHandler := handler.NewAdminPricingHandler(ridePricingService)

	extensionRequestRepo := repository.NewExtensionRequestRepository(pool)
	bookingService := service.NewBookingService(bookingRepo, promotionRepo, pool, assignmentQueueRepo, therapistRepo, offerRepo, serviceRepo, addressRepo, userRepo, messageService, notificationService, extensionRequestRepo, walletService, rideService)
	rideService.SetBookingUpdater(bookingService)
	paymentRepo := repository.NewPaymentRepository(pool)
	paymentService := service.NewPaymentService(paymentRepo)
	bookingHandler := handler.NewBookingHandler(bookingService, paymentService, serviceRepo, addressRepo, therapistRepo, storageService)
	paymentHandler := handler.NewPaymentHandler(paymentService, bookingRepo, serviceRepo, addressRepo)
	promotionService := service.NewPromotionService(promotionRepo)
	promotionHandler := handler.NewPromotionHandler(promotionService)
	reviewRepo := repository.NewReviewRepository(pool)
	reviewService := service.NewReviewService(reviewRepo, notificationService, userRepo)
	clientReviewRepo := repository.NewClientReviewRepository(pool)
	clientReviewService := service.NewClientReviewService(clientReviewRepo)
	reviewHandler := handler.NewReviewHandler(reviewService, clientReviewService, bookingRepo, serviceRepo, userRepo)
	liveLocationRepo := repository.NewLiveLocationRepository(pool)
	liveLocationService := service.NewLiveLocationService(liveLocationRepo, hub)
	liveLocationHandler := handler.NewLiveLocationHandler(liveLocationService)
	emergencyAlertRepo := repository.NewEmergencyAlertRepository(pool)
	emergencyAlertService := service.NewEmergencyAlertService(emergencyAlertRepo)
	emergencyAlertHandler := handler.NewEmergencyAlertHandler(emergencyAlertService, bookingService)
	messageHandler := handler.NewMessageHandler(messageService)
	referralHandler := handler.NewReferralHandler(referralService)
	branchRepo := repository.NewBranchRepository(pool)
	branchService := service.NewBranchService(branchRepo)
	branchHandler := handler.NewBranchHandler(branchService)
<<<<<<< HEAD
	therapistService := service.NewTherapistService(therapistRepo, userRepo)
=======
	therapistService := service.NewTherapistService(therapistRepo)
>>>>>>> 4ccf2642ad97438868848740f3533e97fdbc2996
	therapistHandler := handler.NewTherapistHandler(therapistService, storageService)
	offersHandler := handler.NewOffersHandler(bookingService)
	ticketService := service.NewSupportTicketService(ticketRepo, userRepo)
	ticketHandler := handler.NewSupportTicketHandler(ticketService, storageService)
<<<<<<< HEAD


	
	// Rider Earnings & Safety
	riderWalletService := service.NewRiderWalletService(pool)
	riderWalletHandler := handler.NewRiderWalletHandler(riderWalletService)
	
	// Logistics Module (orchestrates ride creation for bookings)
	logisticsService := service.NewLogisticsService(rideService, bookingRepo, therapistRepo, addressRepo, pool)
	bookingService.SetLogisticsService(logisticsService)
	
	// Wire ride repository to auth handler for rider profile creation
	authHandler.SetRideRepository(rideRepo)
	
=======
>>>>>>> 4ccf2642ad97438868848740f3533e97fdbc2996
	// Start assignment worker with ops notifier to surface critical failures to ops.
	// The notifier will log and, if configured, create a notification for all admins.
	// Runs asynchronously to avoid blocking the caller.
	opsNotifier := func(ctx context.Context, subject string, details map[string]string) error {
		log.Printf("OPS ALERT: %s - %v", subject, details)
<<<<<<< HEAD
		
		// Run the admin notification logic in a separate goroutine to avoid blocking
		go func() {
			// Use a background context since the original may be cancelled
			bgCtx := context.Background()
			
			if userRepo != nil && notificationService != nil {
				// Fetch all admins
				admins, err := userRepo.ListUsers(bgCtx, "admin")
=======
		if userRepo != nil && notificationService != nil {
			// Fetch all admins
			admins, err := userRepo.ListUsers(ctx, "admin")
			if err != nil {
				log.Printf("opsNotifier: failed to list admins: %v", err)
				return err
			}

			// Build a short message from details
			msg := subject
			if len(details) > 0 {
				for k, v := range details {
					msg = msg + "; " + k + "=" + v
				}
			}

			// Notify ALL admins (including offline) via push notification
			for _, admin := range admins {
				_, err := notificationService.Create(ctx, &model.CreateNotificationRequest{
					UserID:  int64(admin.UserID),
					Type:    "ops_alert",
					Title:   "System Alert: " + subject,
					Message: msg,
				})
>>>>>>> 4ccf2642ad97438868848740f3533e97fdbc2996
				if err != nil {
					log.Printf("opsNotifier: failed to list admins: %v", err)
					return
				}

				// Build a short message from details
				msg := subject
				if len(details) > 0 {
					for k, v := range details {
						msg = msg + "; " + k + "=" + v
					}
				}

				// Notify ALL admins (including offline) via push notification
				for _, admin := range admins {
					_, err := notificationService.Create(bgCtx, &model.CreateNotificationRequest{
						UserID:  int64(admin.UserID),
						Type:    "ops_alert",
						Title:   "System Alert: " + subject,
						Message: msg,
					})
					if err != nil {
						log.Printf("failed to create ops notification for admin %d: %v", admin.UserID, err)
					}
				}
			}
		}()
		
		return nil
	}

	// Location/Service Areas (must be initialized before BookingGroupService and AssignmentWorker)
	serviceAreaRepo := repository.NewServiceAreaRepository(pool)
	locationService := service.NewLocationService(serviceAreaRepo)
	locationHandler := handler.NewLocationHandler(locationService)

	// matching service for worker
	therapistMatchingService := service.NewTherapistMatchingService(therapistRepo, bookingRepo)


	// --- Background Workers ---
	// Helper to start worker
	startWorker := func(name string, starter interface{ Start(context.Context) }) {
		workerGroup.Add(1)
		go func() {
			defer workerGroup.Done()
			starter.Start(workerCtx)
		}()
	}

	// Start assignment worker
<<<<<<< HEAD
	assignmentWorker := service.NewAssignmentWorker(pool, assignmentQueueRepo, bookingRepo, paymentRepo, offerRepo, serviceRepo, serviceAreaRepo, therapistRepo, therapistMatchingService, notificationService, opsNotifier)
	startWorker("assignment", assignmentWorker)

	// Start completion worker (auto-completes bookings when timer expires)
	ledgerRepo := repository.NewLedgerRepository(pool)
	// walletService moved up
	completionWorker := service.NewCompletionWorker(pool, bookingRepo, paymentRepo, serviceRepo, ledgerRepo, walletService, notificationService)
	startWorker("completion", completionWorker)

	// Start upcoming booking reminder worker (sends 24h and 2h reminders)
	upcomingBookingWorker := service.NewUpcomingBookingWorker(bookingRepo, notificationService)
	startWorker("upcoming", upcomingBookingWorker)

	// Routing Service
	routingService := service.NewMapboxRoutingService(mapboxToken)

	// Start Rider Dispatch Worker
	riderDispatchWorker := service.NewRiderDispatchWorker(bookingRepo, rideService, routingService, pool)
	startWorker("rider_dispatch", riderDispatchWorker)

	userService := service.NewUserService(userRepo, addressRepo, rideRepo)
	userHandler := handler.NewUserHandler(userService, storageService, authService)
=======
	assignmentWorker := service.NewAssignmentWorker(pool, assignmentQueueRepo, bookingRepo, paymentRepo, offerRepo, serviceRepo, therapistMatchingService, notificationService, opsNotifier)
	assignmentWorker.Start(context.Background())

	// Start completion worker (auto-completes bookings when timer expires)
	completionWorker := service.NewCompletionWorker(pool, bookingRepo, paymentRepo, serviceRepo, notificationService)
	completionWorker.Start(context.Background())

	// Start upcoming booking reminder worker (sends 24h and 2h reminders)
	upcomingBookingWorker := service.NewUpcomingBookingWorker(bookingRepo, notificationService)
	upcomingBookingWorker.Start(context.Background())

	userService := service.NewUserService(userRepo, addressRepo)
	userHandler := handler.NewUserHandler(userService, storageService)
>>>>>>> 4ccf2642ad97438868848740f3533e97fdbc2996
	adminActionRepo := repository.NewAdminActionRepository(pool)
	adminActionService := service.NewAdminActionService(adminActionRepo)
	adminActionHandler := handler.NewAdminActionHandler(adminActionService)
	serviceCache := service.NewServiceCache()
	serviceCatalog := service.NewServiceCatalog(serviceRepo, serviceCache)
	serviceHandler := handler.NewServiceHandler(serviceCatalog, storageService)
	wsHandler := handler.NewWebSocketHandler(hub, cfg.JWTKey)
<<<<<<< HEAD
	reportHandler := handler.NewReportHandler(bookingRepo, ledgerRepo, storageService)



	// Complex Bookings: Product, BookingGroup, BookingAddon repos and service
	productRepo := repository.NewProductRepository(pool)
	productCatalog := service.NewProductCatalog(productRepo, storageService)
	productHandler := handler.NewProductHandler(productCatalog, storageService)
	bookingGroupRepo := repository.NewBookingGroupRepository(pool)
	bookingAddonRepo := repository.NewBookingAddonRepository(pool)
	bookingGroupService := service.NewBookingGroupService(pool, bookingGroupRepo, bookingRepo, bookingAddonRepo, productRepo, serviceRepo, assignmentQueueRepo, addressRepo, locationService)
	bookingGroupHandler := handler.NewBookingGroupHandler(bookingGroupService, productRepo)

	// Shopping Cart
	cartRepo := repository.NewCartRepository(pool)
	cartHandler := handler.NewCartHandler(cartRepo)
=======
	reportHandler := handler.NewReportHandler(bookingRepo)
>>>>>>> 4ccf2642ad97438868848740f3533e97fdbc2996

	// Initialize OAuth configuration
	oauthConfig := &oauth.OAuthProvider{
		Google: &oauth.GoogleConfig{
			ClientID:     cfg.GoogleOAuthClientID,
			ClientSecret: cfg.GoogleOAuthClientSecret,

			CallbackURL:  cfg.GoogleOAuthCallbackURL,
		},
		Apple: &oauth.AppleConfig{
			ClientID:     cfg.AppleOAuthClientID,
			ClientSecret: cfg.AppleOAuthClientSecret,
			CallbackURL:  cfg.AppleOAuthCallbackURL,
		},
	}

	if err := oauth.InitGothProviders(oauthConfig); err != nil {
		log.Printf("Warning: OAuth initialization failed: %v\n", err)
	}

	oauthHandler := handler.NewOAuthHandler(userRepo, cfg.JWTKey, 24*time.Hour)

	// CORS for browser-based development (allow frontend dev server)
	r.Use(cors.Handler(cors.Options{
		// Allow all origins during local development to support socket.io handshakes
		// During local development allow the frontend dev server origin(s).
		AllowedOrigins:   []string{"http://localhost:5173", "http://127.0.0.1:5173", "http://localhost:5174", "http://localhost:5175"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	r.Use(chiMiddleware.Logger)
	
	// Global Rate Limiter (3600 req/min = 60 req/sec, burst 100)
	globalLimiter := middleware.NewGlobalRateLimiter(60, 100)
	r.Use(globalLimiter.Middleware)

	// Request body size limit (1MB default) - security measure against large payloads
	r.Use(middleware.DefaultBodyLimit())

	// Lightweight unauthenticated health endpoints
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{\"status\":\"ok\"}"))
	})
	r.Head("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{\"status\":\"ok\"}"))
		})
		r.Post("/register", authHandler.HandleSignup)
		r.Post("/signup", authHandler.HandleSignup) // Alias for mobile apps
		r.Post("/login", authHandler.HandleLogin)

		// OAuth routes (public)
		r.Get("/oauth/{provider}", oauthHandler.OAuthLoginRequest)
		r.Get("/oauth/callback", oauthHandler.OAuthCallbackRequest)

		// Public Support Tickets (Optional Auth + Rate Limit)
		r.With(func(next http.Handler) http.Handler {
			return middleware.OptionalAuthMiddleware(next, cfg.JWTKey)
		}).With(func(next http.Handler) http.Handler {
			return ticketLimiter.IPRateLimitMiddleware("ticket_create:", next)
		}).Post("/support-tickets", ticketHandler.CreateTicket)

		// Public service catalog listing
		r.Get("/services", serviceHandler.ListServices)

		// Config endpoints (public)
		configHandler := handler.NewConfigHandler()
		r.Get("/config/avatars", configHandler.GetAvatars)

		// Serve static uploads
		fileServer := http.FileServer(http.Dir("./uploads"))
		r.Handle("/uploads/*", http.StripPrefix("/uploads", fileServer))
		// Support HEAD for /services to satisfy HTTP health checks and probes
		r.Head("/services", func(w http.ResponseWriter, r *http.Request) {
			// Call the GET handler to ensure consistent headers, but omit body
			rw := &headResponseWriter{ResponseWriter: w}
			serviceHandler.ListServices(rw, r)
			// Don't write body for HEAD — headResponseWriter ensures no body is sent
		})
		// Public popular and unavailable service lists
		r.Get("/services/popular", serviceHandler.ListPopularServices)
		r.Get("/services/unavailable", serviceHandler.ListUnavailableServices)

		// Expose the WebSocket endpoint at /api/v1/ws and let the handler
		// validate tokens via ?token= for browser clients. It must be
		// registered outside the auth middleware so the middleware does not
		// block the upgrade before the handler can parse the query token.
		r.Get("/ws", wsHandler.HandleConnection)

		// Public Location endpoints (no auth required)
		r.Get("/location/covered", locationHandler.ListCoveredAreas)
		r.With(func(next http.Handler) http.Handler {
			return middleware.OptionalAuthMiddleware(next, cfg.JWTKey)
		}).Post("/location/check", locationHandler.CheckLocation)

		// Apply auth middleware to all subsequent routes in this group
		r.Group(func(r chi.Router) {
			r.Use(func(next http.Handler) http.Handler {
				return middleware.AuthMiddleware(next, cfg.JWTKey)
			})

			r.Route("/users", func(r chi.Router) {
				r.Get("/", userHandler.ListUsers) // internal check: admin only

<<<<<<< HEAD
				r.Group(func(r chi.Router) {
					// Admin-only User Management (Consolidated)
					r.With(func(next http.Handler) http.Handler {
						return middleware.RoleMiddleware([]string{"admin"}, next)
					}).Group(func(r chi.Router) {
						r.Post("/", userHandler.AdminCreateUser)
						r.Patch("/{userID}/status", userHandler.AdminUpdateStatus)
						r.Patch("/{userID}", userHandler.AdminUpdateUserProfile)
						r.Get("/{userId}/addresses", addressHandler.AdminListUserAddresses)
						r.Post("/{userId}/addresses", addressHandler.AdminCreateUserAddress)
					})
=======
			// User profile (authenticated)
			r.Get("/profile", userHandler.GetProfile)
			r.Patch("/profile", userHandler.UpdateProfile)
			r.Post("/users/block", userHandler.BlockUser)
			r.Post("/users/unblock", userHandler.UnblockUser)
			r.Get("/users/blocks", userHandler.GetBlockList)
			r.Post("/users/fcm-token", userHandler.UpdateFCMToken)
			r.Post("/profile/photo", userHandler.UploadProfilePhoto)
>>>>>>> 4ccf2642ad97438868848740f3533e97fdbc2996

					// User profile & utils
					r.Get("/profile", userHandler.GetProfile)
					r.Patch("/profile", userHandler.UpdateProfile)

					// Block/Unblock (Consolidated/RESTful)
					r.Post("/{id}/block", userHandler.BlockUser)
					r.Delete("/{id}/block", userHandler.UnblockUser)
					r.Post("/block", userHandler.BlockUser)     // Shim
					r.Post("/unblock", userHandler.UnblockUser) // Shim

					r.Get("/blocks", userHandler.GetBlockList)
					r.Post("/fcm-token", userHandler.UpdateFCMToken)
					r.Post("/profile/photo", userHandler.UploadProfilePhoto)

					// Favorites
					r.Route("/favorites", func(r chi.Router) {
						r.Get("/", userHandler.ListFavorites)
						r.Post("/{therapist_id}", userHandler.AddFavorite)
						r.Delete("/{therapist_id}", userHandler.RemoveFavorite)
					})
				})
			})

			// Service management (could be limited to admins in the future)
			r.With(func(next http.Handler) http.Handler {
				return middleware.RoleMiddleware([]string{"admin"}, next)
			}).Post("/services", serviceHandler.CreateService)
			r.With(func(next http.Handler) http.Handler {
				return middleware.RoleMiddleware([]string{"admin"}, next)
			}).Post("/services/upload-image", serviceHandler.UploadServiceImage)
<<<<<<< HEAD
			r.With(func(next http.Handler) http.Handler {
				return middleware.RoleMiddleware([]string{"admin"}, next)
			}).Patch("/services/{id}", serviceHandler.UpdateService)
			r.With(func(next http.Handler) http.Handler {
				return middleware.RoleMiddleware([]string{"admin"}, next)
			}).Delete("/services/{id}", serviceHandler.DeleteService)
=======
>>>>>>> 4ccf2642ad97438868848740f3533e97fdbc2996

			// Recent services for authenticated user
			r.Get("/services/recent", serviceHandler.ListRecentServices)

			// User's own support tickets (authenticated)
			r.Get("/support-tickets", ticketHandler.ListMyTickets)

			r.Route("/addresses", func(r chi.Router) {
				r.Post("/", addressHandler.CreateAddress)
				r.Get("/", addressHandler.ListAddresses)
				r.Get("/{id}", addressHandler.GetAddress)
				r.Patch("/{id}", addressHandler.UpdateAddress)
				r.Delete("/{id}", addressHandler.DeleteAddress)
				r.Post("/{id}/default", addressHandler.SetDefaultAddress)
			})

			r.Route("/bookings", func(r chi.Router) {
				r.Post("/", bookingHandler.CreateBooking)
				r.Get("/", bookingHandler.ListBookings)
				r.Get("/{id}", bookingHandler.GetBooking)
				r.Patch("/{id}", bookingHandler.UpdateBooking)
				r.Get("/{id}/extension-request", bookingHandler.GetPendingExtensionRequest)

				// Admin-only Booking Management (Consolidated)
				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"admin"}, next)
				}).Group(func(r chi.Router) {
					r.Post("/admin", bookingHandler.AdminCreateBooking)
					r.Get("/pending", bookingHandler.AdminListPendingBookings)
					r.Get("/{id}/offers", bookingHandler.AdminGetBookingOffers)
					r.Get("/{id}/candidates", bookingHandler.AdminGetBookingCandidates)
					r.Post("/{id}/assign", bookingHandler.AssignTherapist)
				})

				r.Post("/{id}/accept", bookingHandler.AcceptOffer)
				r.Post("/{id}/decline", bookingHandler.DeclineOffer)
				// Legacy routes for admin-mvp compatibility
				r.Post("/{id}/start", bookingHandler.StartBooking)
				r.Post("/{id}/pause", bookingHandler.PauseBooking)
				r.Post("/{id}/resume", bookingHandler.ResumeBooking)
<<<<<<< HEAD
				r.Post("/{id}/complete", bookingHandler.CompleteBooking)

				r.Post("/{id}/payment-proof", bookingHandler.UploadPaymentProof)
				r.Delete("/{id}/payment-proof", bookingHandler.CancelPaymentProof)
				// Therapist/Admin can verify (approve/reject) payment proofs
				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"therapist", "admin"}, next)
				}).Post("/{id}/verify-payment", bookingHandler.VerifyPayment)
				r.Post("/{id}/extend", bookingHandler.ExtendBooking)
				// Extension request accept/reject (therapist only)
				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"therapist", "admin"}, next)
				}).Post("/{id}/extend/accept/{requestId}", bookingHandler.AcceptExtensionRequest)
				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"therapist", "admin"}, next)
				}).Post("/{id}/extend/reject/{requestId}", bookingHandler.RejectExtensionRequest)
				// Client can cancel their own pending extension request
				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"client"}, next)
				}).Post("/{id}/extend/cancel/{requestId}", bookingHandler.CancelExtensionRequest)
				// Therapist or Admin can unassign from a booking
				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"therapist", "admin"}, next)
				}).Post("/{id}/unassign", bookingHandler.UnassignBooking)
			})

			// Complex Bookings: Booking Groups and Products
			r.Route("/booking-groups", func(r chi.Router) {
				r.Post("/", bookingGroupHandler.CreateBookingGroup)
				r.Get("/{id}", bookingGroupHandler.GetBookingGroup)
			})

			r.Route("/products", func(r chi.Router) {
				// Public: list active products and get by ID
				r.Get("/", productHandler.ListProducts)
				r.Get("/{id}", productHandler.GetProduct)
				// Admin-only product management
=======
				r.Patch("/{id}", bookingHandler.UpdateBooking)
				r.Post("/{id}/status", bookingHandler.UpdateBookingStatus)
				r.Post("/{id}/accept", bookingHandler.AcceptOffer)
				r.Post("/{id}/decline", bookingHandler.DeclineOffer)
				r.Post("/{id}/payment-proof", bookingHandler.UploadPaymentProof)
				// Therapist/Admin can verify (approve/reject) payment proofs
				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"therapist", "admin"}, next)
				}).Post("/{id}/verify-payment", bookingHandler.VerifyPayment)
				r.Post("/{id}/extend", bookingHandler.ExtendBooking)
				// Extension request accept/reject (therapist only)
				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"therapist", "admin"}, next)
				}).Post("/{id}/extend/accept/{requestId}", bookingHandler.AcceptExtensionRequest)
				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"therapist", "admin"}, next)
				}).Post("/{id}/extend/reject/{requestId}", bookingHandler.RejectExtensionRequest)
				// Client can cancel their own pending extension request
				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"client"}, next)
				}).Post("/{id}/extend/cancel/{requestId}", bookingHandler.CancelExtensionRequest)
				// Therapist can unassign themselves from a booking
				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"therapist"}, next)
				}).Post("/{id}/unassign", bookingHandler.UnassignBooking)

				// Admin-only route to manually assign a therapist

>>>>>>> 4ccf2642ad97438868848740f3533e97fdbc2996
				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"admin"}, next)
				}).Group(func(r chi.Router) {
					r.Get("/all", productHandler.ListAllProducts)
					r.Post("/", productHandler.CreateProduct)
					r.Put("/{id}", productHandler.UpdateProduct)
					r.Delete("/{id}", productHandler.DeleteProduct)
					r.Post("/upload-image", productHandler.UploadProductImage)
				})
			})

			// Shopping Cart
			r.Route("/cart", func(r chi.Router) {
				r.Get("/", cartHandler.GetCart)
				r.Delete("/", cartHandler.ClearCart)
				r.Post("/items", cartHandler.AddItem)
				r.Put("/items/{itemId}", cartHandler.UpdateItem)
				r.Delete("/items/{itemId}", cartHandler.RemoveItem)
			})

			// Location/Service Areas
			r.Route("/location", func(r chi.Router) {
				r.Post("/check", locationHandler.CheckLocation)
				r.Post("/request-coverage", locationHandler.RequestCoverage)
				r.Get("/covered", locationHandler.ListCoveredAreas)
				// Admin-only routes
				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"admin"}, next)
				}).Get("/demand", locationHandler.ListTopDemand)
				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"admin"}, next)
				}).Patch("/areas/*", locationHandler.UpdateAreaStatus)
				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"admin"}, next)
				}).Post("/areas", locationHandler.CreateServiceArea)
			})

			r.Route("/payments", func(r chi.Router) {
				r.Post("/", paymentHandler.CreatePayment)
				r.Get("/booking/{booking_id}", paymentHandler.GetPaymentByBooking)
				r.Post("/booking/{booking_id}/status", paymentHandler.UpdatePaymentStatus)
			})

			r.Route("/promotions", func(r chi.Router) {
				r.Post("/validate", promotionHandler.ValidatePromotion) // public auth
				
				// Admin-only Promotion Management (Consolidated)
				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"admin"}, next)
				}).Group(func(r chi.Router) {
					r.Post("/", promotionHandler.CreatePromotion)
					r.Get("/", promotionHandler.AdminListPromotions)
					r.Get("/active", promotionHandler.ListActivePromotions)
					r.Get("/code", promotionHandler.GetPromotionByCode)
					r.Patch("/{id}", promotionHandler.UpdatePromotion)
					r.Delete("/{id}", promotionHandler.DeletePromotion)
				})
			})

			r.Route("/reviews", func(r chi.Router) {
				r.Post("/", reviewHandler.CreateReview)
				r.Get("/me", reviewHandler.ListMyReviews)
				r.Get("/booking/{booking_id}", reviewHandler.GetReviewByBooking)
				r.Patch("/{review_id}", reviewHandler.UpdateReview)
				r.Get("/therapist/{therapist_id}", reviewHandler.ListReviewsForTherapist)
				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"therapist"}, next)
				}).Post("/client", reviewHandler.CreateClientReview)
				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"client"}, next)
				}).Get("/client", reviewHandler.ListClientReviews)
			})

			r.Route("/notifications", func(r chi.Router) {
				r.Post("/", notificationHandler.CreateNotification)
				r.Get("/", notificationHandler.ListNotifications)
				r.Put("/read-all", notificationHandler.MarkAllAsRead)
				r.Patch("/{id}", notificationHandler.UpdateNotification)
				r.Post("/{id}/read", notificationHandler.UpdateNotification) // Shim
			})

			// Therapist Wallet (requires therapist role)
			r.Route("/wallet", func(r chi.Router) {
				// Therapist-only: Self management
				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"therapist"}, next)
				}).Group(func(r chi.Router) {
					r.Get("/", walletHandler.GetWallet)
					r.Get("/transactions", walletHandler.GetTransactions)
					r.Post("/payout", walletHandler.RequestPayout)
					r.Get("/payouts", walletHandler.GetPayoutHistory)
				})

				// Admin-only: Global management (Consolidated)
				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"admin"}, next)
				}).Group(func(r chi.Router) {
					r.Get("/payouts/pending", walletHandler.ListPendingPayouts)
					r.Patch("/payouts/{id}", walletHandler.UpdatePayout)
					r.Post("/advances", walletHandler.CreateCashAdvance)
					r.Get("/{therapist_id}", walletHandler.GetTherapistWallet)
				})
			})

			r.Route("/locations", func(r chi.Router) {
				r.Post("/live", liveLocationHandler.UpdateLocation)
				r.Get("/live/{user_id}", liveLocationHandler.GetLocation)
			})

			r.Route("/emergency", func(r chi.Router) {
				r.Post("/trigger", emergencyAlertHandler.TriggerAlert) // public authenticated
				r.Get("/alert/{id}", emergencyAlertHandler.GetAlert)
				r.Post("/alert/{id}/resolve", emergencyAlertHandler.ResolveAlert)

				// Admin-only Dashboard (Consolidated)
				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"admin"}, next)
				}).Group(func(r chi.Router) {
					r.Get("/alerts", emergencyAlertHandler.ListAlerts)
					r.Get("/alerts/count", emergencyAlertHandler.CountAlerts)
				})
			})

			// Admin-only Audit Logs (Consolidated from /admin/actions)
			r.Route("/audit-logs", func(r chi.Router) {
				r.Use(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"admin"}, next)
				})
				r.Post("/", adminActionHandler.LogAction)
				r.Get("/", adminActionHandler.GetAllActions)
				r.Get("/me", adminActionHandler.GetMyActions)
			})

			// Reports & Accounting (Consolidated from /admin/reports)
			r.Route("/reports", func(r chi.Router) {
				r.Use(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"admin"}, next)
				})
				// Accounting
				r.Get("/accounting/summary", reportHandler.GetAccountingSummary)
				r.Get("/accounting/daily", reportHandler.GetDailyAccounting)
				// Ledger
				r.Get("/ledger/summary", reportHandler.GetLedgerSummary)
				r.Get("/ledger/trend", reportHandler.GetLedgerTrend)
				r.Get("/ledger/entries", reportHandler.ListLedgerEntries)
				// Expenses
				r.Route("/expenses", func(r chi.Router) {
					r.Get("/", reportHandler.ListExpenses)
					r.Post("/", reportHandler.CreateExpense)
					r.Post("/upload", reportHandler.UploadExpenseReceipt)
					r.Delete("/{id}", reportHandler.DeleteExpense)
				})
				// Payouts/Settlements
				r.Get("/payouts/balances", reportHandler.ListTherapistBalances)
				r.Post("/payouts/settle", reportHandler.RecordSettlement)
			})

			// Support Tickets (Consolidated from /admin/support-tickets)
			r.Route("/support-tickets", func(r chi.Router) {
				r.Use(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"admin"}, next)
				})
				r.Get("/", ticketHandler.ListTickets)
				r.Patch("/{id}/status", ticketHandler.UpdateTicketStatus)
			})

			// Ride Pricing Configuration (Consolidated from /admin/ride-pricing)
			r.Route("/ride-pricing", func(r chi.Router) {
				r.Use(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"admin"}, next)
				})
				r.Get("/", adminPricingHandler.GetPricingConfig)
				r.Put("/", adminPricingHandler.UpdatePricingConfig)
			})

			// Additional logic to support /rides/active (handled by RiderHandler) - Moved to inside /rides route block


			r.Route("/messages", func(r chi.Router) {
				r.Post("/conversation", messageHandler.CreateConversation)
				r.Get("/conversations", messageHandler.ListConversations)
				r.Post("/send", messageHandler.SendMessage)
				r.Get("/conversation/{conversation_id}", messageHandler.GetMessages)
				r.Post("/message/{message_id}/read", messageHandler.UpdateMessage) // Shim
				r.Patch("/message/{message_id}", messageHandler.UpdateMessage)

				// Admin-only conversation management
				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"admin"}, next)
				}).Group(func(r chi.Router) {
					r.Get("/admin/conversations", messageHandler.ListAllConversations)
					r.Post("/admin/conversations/{conversation_id}/join", messageHandler.AdminJoinConversation)
				})
			})

			r.Route("/referrals", func(r chi.Router) {
				r.Post("/", referralHandler.CreateReferral)
				r.Get("/", referralHandler.ListReferrals)
				r.Get("/code", referralHandler.GetReferralByCode)
				r.Get("/rewards", referralHandler.GetRewards)
				r.Post("/rewards/{reward_id}/redeem", referralHandler.RedeemReward)
			})

			r.Route("/branches", func(r chi.Router) {
				r.Get("/", branchHandler.ListBranches)
				r.Get("/{id}", branchHandler.GetBranch)

				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"admin"}, next)
				}).Post("/", branchHandler.CreateBranch)

				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"admin"}, next)
				}).Patch("/{id}", branchHandler.UpdateBranch)
			})

			// Ride Module Routes
			r.Route("/rides", func(r chi.Router) {
				// Therapist: Request a ride
				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"therapist"}, next)
				}).Post("/request", rideHandler.RequestRide)
				
				// Rider: Accept/Update rides
				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"rider"}, next)
				}).Patch("/{id}", rideHandler.UpdateRide)
				
				// Support for /rides/active
				r.Get("/active", riderHandler.GetActiveRide)
			})

			// Rider Module (offers, location, online status)
			r.Route("/rider", func(r chi.Router) {
				r.Use(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"rider"}, next)
				})
				r.Get("/offers", riderHandler.GetPendingOffers)
				r.Get("/active", riderHandler.GetActiveRide)
				r.Post("/location", riderHandler.UpdateLocation)
				r.Post("/status", riderHandler.UpdateStatus)
				r.Put("/profile", riderHandler.UpdateProfile)
				// Profile creation usually open to auth users or handled separately, 
				// but here we put it under rider group for now (or might need to be outside if role check fails)
				r.Post("/profile", riderHandler.CreateProfile)

				// Wallet & Earnings
				r.Get("/wallet", riderWalletHandler.GetWallet)
				r.Get("/transactions", riderWalletHandler.GetTransactions)
				r.Post("/payout", riderWalletHandler.RequestPayout)
				r.Get("/performance", riderWalletHandler.GetPerformance)
				r.Get("/payout-methods", riderWalletHandler.GetPayoutMethods)
				r.Post("/payout-methods", riderWalletHandler.AddPayoutMethod)
				r.Delete("/payout-methods/{id}", riderWalletHandler.DeletePayoutMethod)
			})
			r.Route("/therapists", func(r chi.Router) {
				r.Get("/", therapistHandler.ListTherapists)
				r.Get("/{id}", therapistHandler.GetProfile)
				r.Get("/{id}/offers", offersHandler.ListForTherapist)
				r.Get("/{id}/services", therapistHandler.GetServices)
				r.Get("/{id}/documents", therapistHandler.GetDocuments)

				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"therapist"}, next)
				}).Patch("/profile", therapistHandler.UpdateProfile)

				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"therapist"}, next)
				}).Post("/documents", therapistHandler.UploadDocument)

				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"therapist"}, next)
				}).Post("/services", therapistHandler.AddService)

				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"therapist"}, next)
				}).Delete("/services/{service_id}", therapistHandler.RemoveService)

				// Therapist location check-in (marks at_branch=true)
				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"therapist"}, next)
				}).Post("/check-in/branch", therapistHandler.CheckInAtBranch)

				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"admin"}, next)
				}).Post("/documents/{document_id}/verify", therapistHandler.VerifyDocument)

				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"admin"}, next)
				}).Patch("/{id}/profile", therapistHandler.AdminUpdateProfile)

				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"admin"}, next)
				}).Put("/{id}/services", therapistHandler.AdminUpdateServices)
			})

			// Admin Shim Block for legacy admin-mvp compatibility (Phase 2 Consolidation)
			r.Route("/admin", func(r chi.Router) {
				r.Use(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"admin"}, next)
				})

				r.Post("/actions", adminActionHandler.LogAction)
				r.Get("/actions", adminActionHandler.GetAllActions)
				r.Get("/actions/me", adminActionHandler.GetMyActions)
				
				r.Get("/users", userHandler.ListUsers)
				r.Patch("/users/{userID}/status", userHandler.AdminUpdateStatus)
				r.Post("/users/{userID}/status", userHandler.AdminUpdateStatus) // Shim
				r.Post("/users", userHandler.AdminCreateUser)
				r.Patch("/users/{userID}", userHandler.AdminUpdateUserProfile)
				r.Get("/users/{userId}/addresses", addressHandler.AdminListUserAddresses)
				r.Post("/users/{userId}/addresses", addressHandler.AdminCreateUserAddress)

				r.Patch("/therapists/{id}", therapistHandler.AdminUpdateProfile)

				r.Post("/bookings", bookingHandler.AdminCreateBooking)
				r.Get("/bookings/pending", bookingHandler.AdminListPendingBookings)
				r.Get("/bookings/{id}/offers", bookingHandler.AdminGetBookingOffers)
				r.Get("/bookings/{id}/candidates", bookingHandler.AdminGetBookingCandidates)
				
				// Booking Status Shims (legacy admin-mvp)
				r.Post("/bookings/{id}/start", bookingHandler.StartBooking)
				r.Post("/bookings/{id}/pause", bookingHandler.PauseBooking)
				r.Post("/bookings/{id}/resume", bookingHandler.ResumeBooking)
				r.Post("/bookings/{id}/complete", bookingHandler.CompleteBooking)
				r.Post("/bookings/{id}/assign", bookingHandler.AssignTherapist)

				r.Get("/support-tickets", ticketHandler.ListTickets)
<<<<<<< HEAD
				r.Patch("/support-tickets/{id}/status", ticketHandler.UpdateTicketStatus)
				r.Post("/support-tickets/{id}/status", ticketHandler.UpdateTicketStatus) // Shim

				r.Get("/reports/accounting/summary", reportHandler.GetAccountingSummary)
				r.Get("/reports/accounting/daily", reportHandler.GetDailyAccounting)
				r.Get("/reports/ledger/summary", reportHandler.GetLedgerSummary)
				r.Get("/reports/ledger/trend", reportHandler.GetLedgerTrend)
				r.Get("/reports/ledger/entries", reportHandler.ListLedgerEntries)
				r.Get("/reports/expenses", reportHandler.ListExpenses)
				r.Post("/reports/expenses", reportHandler.CreateExpense)
				r.Post("/reports/expenses/upload", reportHandler.UploadExpenseReceipt)
				r.Delete("/reports/expenses/{id}", reportHandler.DeleteExpense)
				r.Get("/reports/payouts/balances", reportHandler.ListTherapistBalances)
				r.Post("/reports/payouts/settle", reportHandler.RecordSettlement)

				r.Get("/promotions", promotionHandler.AdminListPromotions)
				r.Patch("/promotions/{id}", promotionHandler.UpdatePromotion)
				r.Delete("/promotions/{id}", promotionHandler.DeletePromotion)

				r.Get("/emergency/alerts", emergencyAlertHandler.ListAlerts)
				r.Get("/emergency/alerts/count", emergencyAlertHandler.CountAlerts)
				r.Post("/emergency/alert/{id}/resolve", emergencyAlertHandler.ResolveAlert) // Shim

				r.Route("/wallet", func(r chi.Router) {
					r.Get("/payouts/pending", walletHandler.ListPendingPayouts)
					r.Post("/payouts/{id}/approve", walletHandler.UpdatePayout) // Shim (body ignored if old app)
					r.Post("/payouts/{id}/reject", walletHandler.UpdatePayout)  // Shim
					r.Patch("/payouts/{id}", walletHandler.UpdatePayout)
					r.Post("/advances", walletHandler.CreateCashAdvance)
					r.Get("/{therapist_id}", walletHandler.GetTherapistWallet)
				})
				
				r.Get("/ride-pricing", adminPricingHandler.GetPricingConfig)
				r.Put("/ride-pricing", adminPricingHandler.UpdatePricingConfig)
=======

				// Accounting/Reporting endpoints
				r.Get("/reports/accounting/summary", reportHandler.GetAccountingSummary)
				r.Get("/reports/accounting/daily", reportHandler.GetDailyAccounting)

				// Emergency Alerts (admin dashboard)
				r.Get("/emergency/alerts", emergencyAlertHandler.ListAlerts)
				r.Get("/emergency/alerts/count", emergencyAlertHandler.CountAlerts)
>>>>>>> 4ccf2642ad97438868848740f3533e97fdbc2996
			})


			// OAuth logout (requires authentication)
			r.Post("/oauth/logout", oauthHandler.OAuthLogout)
		})

		// Other protected routes can go here
	})

	// Create HTTP server with graceful shutdown support
	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	// Channel to listen for OS signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Start server in a goroutine
	go func() {
		fmt.Printf(` ____      _                 _   _               _   _       _     
|  _ \ ___| | __ ___  ____ _| |_(_) ___  _ __   | | | |_   _| |__  
| |_) / _ \ |/ _`+"`"+` \ \/ / _`+"`"+` | __| |/ _ \| '_ \  | |_| | | | | '_ \ 
|  _ <  __/ | (_| |>  < (_| | |_| | (_) | | | | |  _  | |_| | |_) |
|_| \_\___|_|\__,_/_/\_\__,_|\__|_|\___/|_| |_| |_| |_|\__,_|_.__/ 
Server started on port: %s...
`, cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Error starting server: %s\n", err)
		}
	}()

	// Block until shutdown signal is received
	<-stop
	log.Println("Shutting down gracefully... (press Ctrl+C again to force)")

	// 1. Stop HTTP server first to stop accepting new requests
	// Create a deadline for graceful shutdown (30 seconds)
	ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelShutdown()

	if err := server.Shutdown(ctxShutdown); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	// 2. Stop background workers
	log.Println("Stopping background workers...")
	cancelWorkers() // Signal workers to stop

	// Wait for workers to finish their current loop or timeout
	// We use a channel to wait for WaitGroup with timeout
	done := make(chan struct{})
	go func() {
		workerGroup.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("All workers stopped gracefully.")
	case <-time.After(5 * time.Second):
		log.Println("Timeout waiting for workers to stop.")
	}

	// 3. Call explicit Stop() if needed (mostly for logging)
	assignmentWorker.Stop()
	completionWorker.Stop()
	upcomingBookingWorker.Stop()

	// 4. Close DB connection
	log.Println("Closing database connection...")
	db.CloseDB(pool)

	log.Println("Server exited gracefully")
}
