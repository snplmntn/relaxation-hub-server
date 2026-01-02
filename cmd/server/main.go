package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	cors "github.com/go-chi/cors"
	"github.com/snplmntn/relaxation-hub-server/internal/broadcaster"
	"github.com/snplmntn/relaxation-hub-server/internal/config"
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
	config, err := config.LoadConfig()
	if err != nil {
		log.Fatal("Error loading .env file: ", err)
	}
	pool, err := db.InitDB(config.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v\n", err)
	}
	defer db.CloseDB(pool)

	// Initialize legacy WebSocket hub (gorilla) for existing clients
	hub := ws.NewHub()
	hub.SetPool(pool) // Allow hub to perform user enrichment queries
	go hub.Run()

	// Wire the hub into the broadcaster adapter so BroadcastToUser calls work
	broadcaster.SetHub(hub)

	// Create the chi router
	r := chi.NewRouter()

	// Wire dependencies
	userRepo := repository.NewUserRepository(pool)
	authService := service.NewAuthService(userRepo, config)
	rateLimiter := middleware.NewRateLimiter(pool, middleware.DefaultRateLimitConfig())
	ticketLimiter := middleware.NewRateLimiter(pool, middleware.RateLimitConfig{
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

	bookingService := service.NewBookingService(bookingRepo, promotionRepo, pool, assignmentQueueRepo, therapistRepo, offerRepo, serviceRepo, addressRepo, userRepo, messageService, notificationService)
	bookingHandler := handler.NewBookingHandler(bookingService, serviceRepo, addressRepo, therapistRepo)
	paymentRepo := repository.NewPaymentRepository(pool)
	paymentService := service.NewPaymentService(paymentRepo)
	paymentHandler := handler.NewPaymentHandler(paymentService, bookingRepo, serviceRepo, addressRepo)
	promotionService := service.NewPromotionService(promotionRepo)
	promotionHandler := handler.NewPromotionHandler(promotionService)
	reviewRepo := repository.NewReviewRepository(pool)
	reviewService := service.NewReviewService(reviewRepo)
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
	therapistService := service.NewTherapistService(therapistRepo)
	therapistHandler := handler.NewTherapistHandler(therapistService)
	offersHandler := handler.NewOffersHandler(bookingService)
	ticketService := service.NewSupportTicketService(ticketRepo, userRepo)
	ticketHandler := handler.NewSupportTicketHandler(ticketService)
	// Start assignment worker with ops notifier to surface critical failures to ops.
	// The notifier will log and, if configured, create a notification for all admins.
	opsNotifier := func(ctx context.Context, subject string, details map[string]string) error {
		log.Printf("OPS ALERT: %s - %v", subject, details)
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

			for _, admin := range admins {
				// Only notify admins who are currently online (connected via WS)
				if !broadcaster.IsUserOnline(int64(admin.UserID)) {
					continue
				}

				_, err := notificationService.Create(ctx, &model.CreateNotificationRequest{
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
		return nil
	}

	// matching service for worker
	therapistMatchingService := service.NewTherapistMatchingService(therapistRepo, bookingRepo)

	// Start assignment worker
	assignmentWorker := service.NewAssignmentWorker(pool, assignmentQueueRepo, bookingRepo, paymentRepo, offerRepo, therapistMatchingService, notificationService, opsNotifier)
	assignmentWorker.Start(context.Background())

	// Start completion worker (auto-completes bookings when timer expires)
	completionWorker := service.NewCompletionWorker(pool, bookingRepo, notificationService)
	completionWorker.Start(context.Background())
	userService := service.NewUserService(userRepo, addressRepo)
	userHandler := handler.NewUserHandler(userService)
	adminActionRepo := repository.NewAdminActionRepository(pool)
	adminActionService := service.NewAdminActionService(adminActionRepo)
	adminActionHandler := handler.NewAdminActionHandler(adminActionService)
	serviceCache := service.NewServiceCache()
	serviceCatalog := service.NewServiceCatalog(serviceRepo, serviceCache)
	serviceHandler := handler.NewServiceHandler(serviceCatalog)
	wsHandler := handler.NewWebSocketHandler(hub, config.JWTKey)

	// Initialize OAuth configuration
	oauthConfig := &oauth.OAuthProvider{
		Google: &oauth.GoogleConfig{
			ClientID:     config.GoogleOAuthClientID,
			ClientSecret: config.GoogleOAuthClientSecret,
			CallbackURL:  config.GoogleOAuthCallbackURL,
		},
		Apple: &oauth.AppleConfig{
			ClientID:     config.AppleOAuthClientID,
			ClientSecret: config.AppleOAuthClientSecret,
			CallbackURL:  config.AppleOAuthCallbackURL,
		},
	}

	if err := oauth.InitGothProviders(oauthConfig); err != nil {
		log.Printf("Warning: OAuth initialization failed: %v\n", err)
	}

	oauthHandler := handler.NewOAuthHandler(userRepo, config.JWTKey, 24*time.Hour)

	// CORS for browser-based development (allow frontend dev server)
	r.Use(cors.Handler(cors.Options{
		// Allow all origins during local development to support socket.io handshakes
		// During local development allow the frontend dev server origin(s).
		AllowedOrigins:   []string{"http://localhost:5173", "http://127.0.0.1:5173", "http://localhost:5174"},
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
		r.Post("/login", authHandler.HandleLogin)

		// OAuth routes (public)
		r.Get("/oauth/{provider}", oauthHandler.OAuthLoginRequest)
		r.Get("/oauth/callback", oauthHandler.OAuthCallbackRequest)

		// Public Support Tickets (Optional Auth + Rate Limit)
		r.With(func(next http.Handler) http.Handler {
			return middleware.OptionalAuthMiddleware(next, config.JWTKey)
		}).With(func(next http.Handler) http.Handler {
			return ticketLimiter.IPRateLimitMiddleware("ticket_create:", next)
		}).Post("/support-tickets", ticketHandler.CreateTicket)

		// Public service catalog listing
		r.Get("/services", serviceHandler.ListServices)

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

		// Apply auth middleware to all subsequent routes in this group
		r.Group(func(r chi.Router) {
			r.Use(func(next http.Handler) http.Handler {
				return middleware.AuthMiddleware(next, config.JWTKey)
			})

			// Expose a users list endpoint for clients to discover chat targets
			r.Get("/users", userHandler.ListUsers)

			// User profile (authenticated)
			r.Get("/profile", userHandler.GetProfile)
			r.Patch("/profile", userHandler.UpdateProfile)
			r.Post("/users/block", userHandler.BlockUser)
			r.Post("/users/unblock", userHandler.UnblockUser)
			r.Get("/users/blocks", userHandler.GetBlockList)
			r.Post("/users/fcm-token", userHandler.UpdateFCMToken)

			// Favorites endpoints
			r.Get("/users/favorites", userHandler.ListFavorites)
			r.Post("/users/favorites/{therapist_id}", userHandler.AddFavorite)
			r.Delete("/users/favorites/{therapist_id}", userHandler.RemoveFavorite)

			// Service management (could be limited to admins in the future)
			r.With(func(next http.Handler) http.Handler {
				return middleware.RoleMiddleware([]string{"admin"}, next)
			}).Post("/services", serviceHandler.CreateService)

			// Recent services for authenticated user
			r.Get("/services/recent", serviceHandler.ListRecentServices)

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
				r.Post("/{id}/start", bookingHandler.StartBooking)
				r.Post("/{id}/pause", bookingHandler.PauseBooking)
				r.Post("/{id}/resume", bookingHandler.ResumeBooking)
				r.Patch("/{id}", bookingHandler.UpdateBooking)
				r.Post("/{id}/status", bookingHandler.UpdateBookingStatus)
				r.Post("/{id}/accept", bookingHandler.AcceptOffer)
				r.Post("/{id}/decline", bookingHandler.DeclineOffer)

				// Admin-only route to manually assign a therapist
				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"admin"}, next)
				}).Post("/{id}/assign", bookingHandler.AssignTherapist)
			})

			r.Route("/payments", func(r chi.Router) {
				r.Post("/", paymentHandler.CreatePayment)
				r.Get("/booking/{booking_id}", paymentHandler.GetPaymentByBooking)
				r.Post("/booking/{booking_id}/status", paymentHandler.UpdatePaymentStatus)
			})

			r.Route("/promotions", func(r chi.Router) {
				r.Post("/", promotionHandler.CreatePromotion)
				r.Get("/", promotionHandler.ListActivePromotions)
				r.Get("/code", promotionHandler.GetPromotionByCode)
				r.Post("/validate", promotionHandler.ValidatePromotion)
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
				r.Post("/{id}/read", notificationHandler.MarkNotificationAsRead)
			})

			r.Route("/locations", func(r chi.Router) {
				r.Post("/live", liveLocationHandler.UpdateLocation)
				r.Get("/live/{user_id}", liveLocationHandler.GetLocation)
			})

			r.Route("/emergency", func(r chi.Router) {
				r.Post("/trigger", emergencyAlertHandler.TriggerAlert)
				r.Get("/alert/{id}", emergencyAlertHandler.GetAlert)
				r.Post("/alert/{id}/resolve", emergencyAlertHandler.ResolveAlert)
			})

			r.Route("/messages", func(r chi.Router) {
				r.Post("/conversation", messageHandler.CreateConversation)
				r.Get("/conversations", messageHandler.ListConversations)
				r.Post("/send", messageHandler.SendMessage)
				r.Get("/conversation/{conversation_id}", messageHandler.GetMessages)
				r.Post("/message/{message_id}/read", messageHandler.MarkMessageAsRead)
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

				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"admin"}, next)
				}).Post("/documents/{document_id}/verify", therapistHandler.VerifyDocument)
			})

			r.Route("/admin", func(r chi.Router) {
				r.Use(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"admin"}, next)
				})

				r.Post("/actions", adminActionHandler.LogAction)
				r.Get("/actions", adminActionHandler.GetAllActions)
				r.Get("/actions/me", adminActionHandler.GetMyActions)

				// Admin: create bookings on behalf of clients
				r.Post("/bookings", bookingHandler.AdminCreateBooking)

				// Admin intervention: pending bookings, offers, and candidate therapists
				r.Get("/bookings/pending", bookingHandler.AdminListPendingBookings)
				r.Get("/bookings/{id}/offers", bookingHandler.AdminGetBookingOffers)
				r.Get("/bookings/{id}/candidates", bookingHandler.AdminGetBookingCandidates)

				r.Get("/support-tickets", ticketHandler.ListTickets)
			})


			// OAuth logout (requires authentication)
			r.Post("/oauth/logout", oauthHandler.OAuthLogout)
		})

		// Other protected routes can go here
	})

	fmt.Printf("Starting Relaxation Hub Server on port: %s...\n", config.Port)
	if err := http.ListenAndServe(":"+config.Port, r); err != nil {
		fmt.Printf("Error starting server: %s\n", err)
	}
}
