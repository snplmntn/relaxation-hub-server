package app

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	cors "github.com/go-chi/cors"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
)

func registerRoutes(r chi.Router, deps *dependencies) {
	// CORS for browser-based development (allow frontend dev server)
	r.Use(cors.Handler(cors.Options{
		// Allow all origins during local development to support socket.io handshakes
		// During local development allow the frontend dev server origin(s).
		AllowedOrigins: []string{
			"http://localhost:5173",
			"http://127.0.0.1:5173",
			"http://localhost:5174",
			"http://127.0.0.1:5174",
			"http://bookhiraya.netlify.app",
			"https://bookhiraya.com",
			"https://www.bookhiraya.com",
			"https://staff.bookhiraya.com",
			"https://staffhiraya.netlify.app",
			"https://hirayahomespa.ph",
			"https://www.hirayahomespa.ph",
			"https://kalingaspa.com",
			"https://www.kalingaspa.com",
			"https://staff.kalingaspa.com",
		},
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
	healthHandler := NewHealthHandler()
	r.Get("/health", healthHandler)
	r.Head("/health", func(w http.ResponseWriter, r *http.Request) {
		rw := &headResponseWriter{ResponseWriter: w}
		healthHandler(rw, r)
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/health", healthHandler)
		r.Post("/register", deps.authHandler.HandleSignup)
		r.Post("/signup", deps.authHandler.HandleSignup) // Alias for mobile apps
		r.Post("/login", deps.authHandler.HandleLogin)

		// OAuth routes (public)
		r.Get("/oauth/{provider}", deps.oauthHandler.OAuthLoginRequest)
		r.Get("/oauth/callback", deps.oauthHandler.OAuthCallbackRequest)
		r.With(func(next http.Handler) http.Handler {
			return deps.authLimiter.IPRateLimitMiddleware("google_auth:", next)
		}).Post("/oauth/google/credential", deps.googleAuthHandler.Authenticate)

		// Public Support Tickets (Optional Auth + Rate Limit)
		r.With(func(next http.Handler) http.Handler {
			return middleware.OptionalAuthMiddleware(next, deps.cfg.JWTKey)
		}).With(func(next http.Handler) http.Handler {
			return deps.ticketLimiter.IPRateLimitMiddleware("ticket_create:", next)
		}).Post("/support-tickets", deps.ticketHandler.CreateTicket)

		// PayMongo webhook. Public by necessity — PayMongo cannot carry our JWT
		// — and authenticated instead by the HMAC signature over the raw body,
		// which the handler verifies before interpreting anything.
		if deps.bookingCheckoutHandler != nil {
			r.Post("/webhooks/paymongo", deps.bookingCheckoutHandler.HandlePayMongoWebhook)
		}

		// Public service catalog listing
		r.Get("/services", deps.serviceHandler.ListServices)

		// Config endpoints (public)
		configHandler := deps.configHandler
		r.Get("/config/avatars", configHandler.GetAvatars)
		r.Get("/public/legal/{docKey}", deps.legalDocHandler.GetLegalDocument)
		r.Get("/content/{key}", deps.legalDocHandler.GetContentPage)

		// Serve static uploads
		fileServer := http.FileServer(http.Dir("./uploads"))
		r.Handle("/uploads/*", http.StripPrefix("/uploads", fileServer))
		// Support HEAD for /services to satisfy HTTP health checks and probes
		r.Head("/services", func(w http.ResponseWriter, r *http.Request) {
			// Call the GET handler to ensure consistent headers, but omit body
			rw := &headResponseWriter{ResponseWriter: w}
			deps.serviceHandler.ListServices(rw, r)
			// Don't write body for HEAD — headResponseWriter ensures no body is sent
		})
		// Public popular and unavailable service lists
		r.Get("/services/popular", deps.serviceHandler.ListPopularServices)
		r.Get("/services/unavailable", deps.serviceHandler.ListUnavailableServices)

		// Expose the WebSocket endpoint at /api/v1/ws and let the handler
		// validate tokens via ?token= for browser clients. It must be
		// registered outside the auth middleware so the middleware does not
		// block the upgrade before the handler can parse the query token.
		r.Get("/ws", deps.wsHandler.HandleConnection)

		// Public Location endpoints (no auth required)
		r.Get("/location/covered", deps.locationHandler.ListCoveredAreas)
		r.With(func(next http.Handler) http.Handler {
			return middleware.OptionalAuthMiddleware(next, deps.cfg.JWTKey)
		}).Post("/location/check", deps.locationHandler.CheckLocation)
		r.Get("/availability", deps.availabilityHandler.CheckAvailability)
		r.Get("/public/branches", deps.branchHandler.ListPublicBranches)
		r.Post("/applications", deps.applicationHandler.Submit)
		r.Get("/blog-posts", deps.blogPostHandler.ListPublished)
		r.Get("/blog-posts/{slug}", deps.blogPostHandler.GetPublishedBySlug)
		r.Get("/blog-assets/{filename}", deps.blogPostHandler.GetAsset)
		r.Get("/landing-settings", deps.landingSettingsHandler.GetLandingSettings)

		// Apply auth middleware to all subsequent routes in this group
		r.Group(func(r chi.Router) {
			r.Use(func(next http.Handler) http.Handler {
				return middleware.AuthMiddleware(next, deps.cfg.JWTKey)
			})
			r.Use(middleware.NewAccountStatusMiddleware(deps.userRepo))
			r.Post("/oauth/google/link", deps.googleAuthHandler.Link)

			r.Post("/availability/booking", deps.availabilityHandler.CheckBookingAvailability)

			r.Route("/users", func(r chi.Router) {
				r.Get("/", deps.userHandler.ListUsers) // internal check: admin only

				r.Group(func(r chi.Router) {
					// Admin-only User Management (Consolidated)
					r.With(func(next http.Handler) http.Handler {
						return middleware.RoleMiddleware(middleware.AdminOperationalRoles, next)
					}).Group(func(r chi.Router) {
						r.Post("/", deps.userHandler.AdminCreateUser)
						r.Get("/stats", deps.userHandler.GetUserStats)
						r.Get("/export", deps.userHandler.AdminExportUsers)
						r.Patch("/{userID}/status", deps.userHandler.AdminUpdateOperationalUserStatus)
						r.Patch("/{userID}", deps.userHandler.AdminUpdateOperationalUserProfile)
						r.Get("/{userId}/addresses", deps.addressHandler.AdminListUserAddresses)
						r.Post("/{userId}/addresses", deps.addressHandler.AdminCreateUserAddress)
						r.Patch("/{userId}/addresses/{id}", deps.addressHandler.AdminUpdateUserAddress)
						r.Delete("/{userId}/addresses/{id}", deps.addressHandler.AdminDeleteUserAddress)
						r.Post("/{userId}/addresses/{id}/default", deps.addressHandler.AdminSetDefaultUserAddress)
						r.Post("/{userId}/addresses/{id}/disable", deps.addressHandler.AdminDisableUserAddress)
						r.Post("/{userId}/addresses/{id}/enable", deps.addressHandler.AdminEnableUserAddress)
					})

					// User profile & utils
					r.Get("/profile", deps.userHandler.GetProfile)
					r.Patch("/profile", deps.userHandler.UpdateProfile)
					r.Delete("/profile", deps.accountSecurityHandler.DeleteAccount)
					r.Patch("/profile/password", deps.accountSecurityHandler.ChangePassword)

					// Block/Unblock (Consolidated/RESTful)
					r.Post("/{id}/block", deps.userHandler.BlockUser)
					r.Delete("/{id}/block", deps.userHandler.UnblockUser)
					r.Post("/block", deps.userHandler.BlockUser)     // Shim
					r.Post("/unblock", deps.userHandler.UnblockUser) // Shim

					r.Get("/blocks", deps.userHandler.GetBlockList)
					r.Post("/fcm-token", deps.userHandler.UpdateFCMToken)
					r.Post("/profile/photo", deps.userHandler.UploadProfilePhoto)

					// Favorites
					r.Route("/favorites", func(r chi.Router) {
						r.Get("/", deps.userHandler.ListFavorites)
						r.Post("/{therapist_id}", deps.userHandler.AddFavorite)
						r.Delete("/{therapist_id}", deps.userHandler.RemoveFavorite)
					})
				})
			})

			r.Route("/staff", func(r chi.Router) {
				r.Use(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware(middleware.SuperAdminOnlyRoles, next)
				})
				r.Get("/", deps.userHandler.ListStaff)
				r.Post("/", deps.userHandler.AdminCreateStaff)
				r.Patch("/{userID}", deps.userHandler.AdminUpdateStaffProfile)
				r.Patch("/{userID}/status", deps.userHandler.AdminUpdateStaffStatus)
				r.Patch("/{userID}/password", deps.accountSecurityHandler.ResetStaffPassword)
			})

			// Service management (could be limited to admins in the future)
			r.With(func(next http.Handler) http.Handler {
				return middleware.RoleMiddleware(middleware.SuperAdminOnlyRoles, next)
			}).Post("/services", deps.serviceHandler.CreateService)
			r.With(func(next http.Handler) http.Handler {
				return middleware.RoleMiddleware(middleware.SuperAdminOnlyRoles, next)
			}).Post("/services/upload-image", deps.serviceHandler.UploadServiceImage)
			r.With(func(next http.Handler) http.Handler {
				return middleware.RoleMiddleware(middleware.SuperAdminOnlyRoles, next)
			}).Patch("/services/{id}", deps.serviceHandler.UpdateService)
			r.With(func(next http.Handler) http.Handler {
				return middleware.RoleMiddleware(middleware.SuperAdminOnlyRoles, next)
			}).Delete("/services/{id}", deps.serviceHandler.DeleteService)

			// Landing page settings (super admin only)
			r.With(func(next http.Handler) http.Handler {
				return middleware.RoleMiddleware(middleware.SuperAdminOnlyRoles, next)
			}).Patch("/landing-settings", deps.landingSettingsHandler.UpdateLandingSettings)

			// Recent services for authenticated user
			r.Get("/services/recent", deps.serviceHandler.ListRecentServices)

			// User's own support tickets (authenticated)
			r.Get("/support-tickets", deps.ticketHandler.ListMyTickets)

			r.Route("/addresses", func(r chi.Router) {
				r.Post("/", deps.addressHandler.CreateAddress)
				r.Get("/", deps.addressHandler.ListAddresses)
				r.Get("/{id}", deps.addressHandler.GetAddress)
				r.Patch("/{id}", deps.addressHandler.UpdateAddress)
				r.Delete("/{id}", deps.addressHandler.DeleteAddress)
				r.Post("/{id}/default", deps.addressHandler.SetDefaultAddress)
			})

			r.Route("/bookings", func(r chi.Router) {
				r.Post("/", deps.bookingHandler.CreateBooking)
				r.Get("/", deps.bookingHandler.ListBookings)
				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware(middleware.AdminOperationalRoles, next)
				}).Get("/export.xlsx", deps.bookingHandler.ExportBookingReportWorkbook)
				r.Get("/{id}", deps.bookingHandler.GetBooking)
				r.Get("/{id}/live-location", deps.liveLocationHandler.GetBookingLocation)
				r.Patch("/{id}", deps.bookingHandler.UpdateBooking)
				r.Get("/{id}/extension-request", deps.bookingHandler.GetPendingExtensionRequest)

				// Admin-only Booking Management (Consolidated)
				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware(middleware.AdminOperationalRoles, next)
				}).Group(func(r chi.Router) {
					r.Post("/admin", deps.bookingHandler.AdminCreateBooking)
					r.Get("/pending", deps.bookingHandler.AdminListPendingBookings)
					r.Get("/{id}/offers", deps.bookingHandler.AdminGetBookingOffers)
					r.Get("/{id}/candidates", deps.bookingHandler.AdminGetBookingCandidates)
					r.Post("/{id}/assign", deps.bookingHandler.AssignTherapist)
					r.Post("/{id}/riders/assign", deps.bookingHandler.AssignRider)
				})

				r.Post("/{id}/accept", deps.bookingHandler.AcceptOffer)
				r.Post("/{id}/decline", deps.bookingHandler.DeclineOffer)
				// Legacy routes for admin-mvp compatibility
				r.Post("/{id}/start", deps.bookingHandler.StartBooking)
				r.Post("/{id}/pause", deps.bookingHandler.PauseBooking)
				r.Post("/{id}/resume", deps.bookingHandler.ResumeBooking)
				r.Post("/{id}/complete", deps.bookingHandler.CompleteBooking)

				r.Post("/{id}/payment-proof", deps.bookingHandler.UploadPaymentProof)
				r.Delete("/{id}/payment-proof", deps.bookingHandler.CancelPaymentProof)
				// Therapist/Admin can verify (approve/reject) payment proofs
				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"therapist", "admin", "super_admin"}, next)
				}).Post("/{id}/verify-payment", deps.bookingHandler.VerifyPayment)
				r.Post("/{id}/extend", deps.bookingHandler.ExtendBooking)
				// Extension request accept/reject (therapist only)
				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"therapist", "admin", "super_admin"}, next)
				}).Post("/{id}/extend/accept/{requestId}", deps.bookingHandler.AcceptExtensionRequest)
				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"therapist", "admin", "super_admin"}, next)
				}).Post("/{id}/extend/reject/{requestId}", deps.bookingHandler.RejectExtensionRequest)
				// Client can cancel their own pending extension request
				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"client"}, next)
				}).Post("/{id}/extend/cancel/{requestId}", deps.bookingHandler.CancelExtensionRequest)
				// Therapist or Admin can unassign from a booking
				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"therapist", "admin", "super_admin"}, next)
				}).Post("/{id}/unassign", deps.bookingHandler.UnassignBooking)
			})

			r.With(func(next http.Handler) http.Handler {
				return middleware.RoleMiddleware(middleware.AdminOperationalRoles, next)
			}).Get("/booking-events", deps.bookingHandler.HandleListAllEvents)

			r.Route("/attendance", func(r chi.Router) {
				r.Use(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware(middleware.AdminOperationalRoles, next)
				})
				r.Get("/staff", deps.staffAttendanceHandler.ListStaffAttendanceAdminTargets)
				r.Get("/", deps.staffAttendanceHandler.ListStaffAttendance)
				r.Post("/", deps.staffAttendanceHandler.CreateStaffAttendance)
				r.Patch("/{id}", deps.staffAttendanceHandler.UpdateStaffAttendance)
				r.Delete("/{id}", deps.staffAttendanceHandler.DeleteStaffAttendance)
			})

			r.Route("/payroll", func(r chi.Router) {
				r.Use(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware(middleware.SuperAdminOnlyRoles, next)
				})
				r.Post("/runs", deps.payrollHandler.CreatePayrollRun)
				r.Get("/runs", deps.payrollHandler.ListPayrollRuns)
				r.Get("/runs/{id}", deps.payrollHandler.GetPayrollRun)
				r.Get("/runs/{id}/export.xlsx", deps.payrollHandler.ExportPayrollWorkbook)
				r.Get("/runs/{id}/export.pdf", deps.payrollHandler.ExportPayrollPayslipPDF)
				r.Post("/runs/{id}/approve", deps.payrollHandler.ApprovePayrollRun)
				r.Post("/runs/{id}/void", deps.payrollHandler.VoidPayrollRun)
				r.Post("/runs/{id}/rows/{rowID}/mark-paid", deps.payrollHandler.MarkPayrollRowPaid)
				r.Get("/runs/{id}/staleness", deps.payrollHandler.CheckPayrollRunStaleness)
				r.Get("/rates", deps.payrollHandler.ListCompensationRates)
				r.Post("/rates", deps.payrollHandler.CreateCompensationRate)
				r.Get("/adjustments", deps.payrollHandler.ListStaffPayrollAdjustments)
				r.Post("/adjustments", deps.payrollHandler.CreateStaffPayrollAdjustment)
				r.Patch("/adjustments/{id}", deps.payrollHandler.UpdateStaffPayrollAdjustment)
				r.Delete("/adjustments/{id}", deps.payrollHandler.DeleteStaffPayrollAdjustment)
				r.Put("/staff-profiles/{userID}", deps.payrollHandler.UpsertStaffProfile)
			})

			// Therapist cash on hand + remittance (admin + super admin)
			r.With(func(next http.Handler) http.Handler {
				return middleware.RoleMiddleware(middleware.AdminOperationalRoles, next)
			}).Route("/cash-remittances", func(r chi.Router) {
				r.Get("/on-hand", deps.cashRemittanceHandler.ListCashOnHand)
				r.Get("/logs", deps.cashRemittanceHandler.ListRemittanceLog)
				r.Get("/", deps.cashRemittanceHandler.ListHistory)
				r.Post("/", deps.cashRemittanceHandler.CreateRemittance)
			})

			// Daily accounting sheet line items (expenses + therapist tips).
			// Gated to super admin to match /reports, which reads the same data.
			r.With(func(next http.Handler) http.Handler {
				return middleware.RoleMiddleware(middleware.SuperAdminOnlyRoles, next)
			}).Route("/accounting", func(r chi.Router) {
				r.Get("/expenses", deps.accountingHandler.ListExpenses)
				r.Post("/expenses", deps.accountingHandler.CreateExpense)
				r.Delete("/expenses/{id}", deps.accountingHandler.DeleteExpense)
				r.Get("/tips", deps.accountingHandler.ListTips)
				r.Post("/tips", deps.accountingHandler.CreateTip)
				r.Delete("/tips/{id}", deps.accountingHandler.DeleteTip)
			})

			r.With(func(next http.Handler) http.Handler {
				return middleware.RoleMiddleware(middleware.AdminOperationalRoles, next)
			}).Route("/day-view/therapist-order", func(r chi.Router) {
				r.Get("/", deps.dayViewOrderHandler.GetTherapistOrder)
				r.Put("/", deps.dayViewOrderHandler.UpdateTherapistOrder)
			})

			// Complex Bookings: Booking Groups and Products
			r.Route("/booking-groups", func(r chi.Router) {
				r.Post("/validate-voucher", deps.bookingGroupHandler.PreviewVoucher)
				r.Post("/", deps.bookingGroupHandler.CreateBookingGroup)
				r.Get("/{id}", deps.bookingGroupHandler.GetBookingGroup)
			})

			// Online payment. A checkout creates no booking until PayMongo
			// confirms; see the webhook route registered publicly below.
			if deps.bookingCheckoutHandler != nil {
				r.Route("/checkouts", func(r chi.Router) {
					r.Post("/", deps.bookingCheckoutHandler.StartCheckout)
					r.Get("/{reference}", deps.bookingCheckoutHandler.GetCheckoutStatus)
				})
			}
			r.With(func(next http.Handler) http.Handler {
				return middleware.RoleMiddleware(middleware.AdminOperationalRoles, next)
			}).Post("/booking-groups/admin", deps.bookingGroupHandler.CreateBookingGroupAsAdmin)

			// Recurring Bookings (admin only)
			r.With(func(next http.Handler) http.Handler {
				return middleware.RoleMiddleware(middleware.AdminOperationalRoles, next)
			}).Route("/recurring-bookings", func(r chi.Router) {
				r.Post("/", deps.recurringBookingHandler.Create)
				r.Get("/", deps.recurringBookingHandler.List)
				r.Get("/{id}", deps.recurringBookingHandler.GetByID)
				r.Patch("/{id}", deps.recurringBookingHandler.Update)
			})

			r.Route("/products", func(r chi.Router) {
				// Public: list active products and get by ID
				r.Get("/", deps.productHandler.ListProducts)
				r.Get("/{id}", deps.productHandler.GetProduct)
				// Admin-only product management
				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware(middleware.SuperAdminOnlyRoles, next)
				}).Group(func(r chi.Router) {
					r.Get("/all", deps.productHandler.ListAllProducts)
					r.Post("/", deps.productHandler.CreateProduct)
					r.Put("/{id}", deps.productHandler.UpdateProduct)
					r.Delete("/{id}", deps.productHandler.DeleteProduct)
					r.Post("/upload-image", deps.productHandler.UploadProductImage)
				})
			})

			// Shopping Cart
			r.Route("/cart", func(r chi.Router) {
				r.Get("/", deps.cartHandler.GetCart)
				r.Delete("/", deps.cartHandler.ClearCart)
				r.Post("/items", deps.cartHandler.AddItem)
				r.Put("/items/{itemId}", deps.cartHandler.UpdateItem)
				r.Delete("/items/{itemId}", deps.cartHandler.RemoveItem)
			})

			// Location/Service Areas
			r.Route("/location", func(r chi.Router) {
				r.Post("/check", deps.locationHandler.CheckLocation)
				r.Post("/request-coverage", deps.locationHandler.RequestCoverage)
				r.Get("/covered", deps.locationHandler.ListCoveredAreas)
				// Admin-only routes
				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware(middleware.AdminOperationalRoles, next)
				}).Get("/demand", deps.locationHandler.ListTopDemand)
				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware(middleware.AdminOperationalRoles, next)
				}).Patch("/areas/{area_key}", deps.locationHandler.UpdateAreaStatus)
				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware(middleware.AdminOperationalRoles, next)
				}).Get("/areas/{area_key}/interested-users", deps.locationHandler.ListInterestedUsers)
				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware(middleware.AdminOperationalRoles, next)
				}).Post("/areas", deps.locationHandler.CreateServiceArea)
				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware(middleware.AdminOperationalRoles, next)
				}).Get("/areas", deps.locationHandler.ListAllAreas)
			})

			r.Route("/payments", func(r chi.Router) {
				r.Post("/", deps.paymentHandler.CreatePayment)
				r.Get("/booking/{booking_id}", deps.paymentHandler.GetPaymentByBooking)
				r.Post("/booking/{booking_id}/status", deps.paymentHandler.UpdatePaymentStatus)
			})

			r.Route("/promotions", func(r chi.Router) {
				r.Post("/validate", deps.promotionHandler.ValidatePromotion) // public auth
				r.Get("/active", deps.promotionHandler.ListActivePromotions)

				// Admin-only Promotion Management (Consolidated)
				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware(middleware.SuperAdminOnlyRoles, next)
				}).Group(func(r chi.Router) {
					r.Post("/", deps.promotionHandler.CreatePromotion)
					r.Get("/", deps.promotionHandler.AdminListPromotions)
					r.Get("/code", deps.promotionHandler.GetPromotionByCode)
					r.Patch("/{id}", deps.promotionHandler.UpdatePromotion)
					r.Delete("/{id}", deps.promotionHandler.DeletePromotion)
				})
			})

			r.Route("/content", func(r chi.Router) {
				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"super_admin"}, next)
				}).Put("/{key}", deps.legalDocHandler.UpdateContentPage)
			})

			r.Route("/admin/blog-posts", func(r chi.Router) {
				r.Use(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware(middleware.AdminOperationalRoles, next)
				})
				r.Get("/", deps.blogPostHandler.ListAdmin)
				r.Post("/", deps.blogPostHandler.Create)
				r.Post("/upload-cover", deps.blogPostHandler.UploadCover)
				r.Delete("/assets/{filename}", deps.blogPostHandler.DeleteAsset)
				r.Patch("/{id}", deps.blogPostHandler.Update)
				r.Delete("/{id}", deps.blogPostHandler.Delete)
			})

			r.Route("/reviews", func(r chi.Router) {
				r.Post("/", deps.reviewHandler.CreateReview)
				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware(middleware.AdminOperationalRoles, next)
				}).Get("/", deps.reviewHandler.AdminListReviews)
				r.Get("/me", deps.reviewHandler.ListMyReviews)
				r.Get("/booking/{booking_id}", deps.reviewHandler.GetReviewByBooking)
				r.Patch("/{review_id}", deps.reviewHandler.UpdateReview)
				r.Get("/therapist/{therapist_id}", deps.reviewHandler.ListReviewsForTherapist)
				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"therapist"}, next)
				}).Post("/client", deps.reviewHandler.CreateClientReview)
				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"client"}, next)
				}).Get("/client", deps.reviewHandler.ListClientReviews)
			})

			r.Route("/notifications", func(r chi.Router) {
				r.Post("/", deps.notificationHandler.CreateNotification)
				r.Get("/", deps.notificationHandler.ListNotifications)
				r.Get("/unread-count", deps.notificationHandler.CountUnread)
				r.Put("/read-all", deps.notificationHandler.MarkAllAsRead)
				r.Patch("/{id}", deps.notificationHandler.UpdateNotification)
				r.Post("/{id}/read", deps.notificationHandler.UpdateNotification) // Shim
			})

			// Therapist Wallet (requires therapist role)
			r.Route("/wallet", func(r chi.Router) {
				// Therapist-only: Self management
				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"therapist"}, next)
				}).Group(func(r chi.Router) {
					r.Get("/", deps.walletHandler.GetWallet)
					r.Get("/transactions", deps.walletHandler.GetTransactions)
					r.Post("/payout", deps.walletHandler.RequestPayout)
					r.Get("/payouts", deps.walletHandler.GetPayoutHistory)
				})

				// Admin-only: Global management (Consolidated)
				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware(middleware.SuperAdminOnlyRoles, next)
				}).Group(func(r chi.Router) {
					r.Get("/payouts/pending", deps.walletHandler.ListPendingPayouts)
					r.Patch("/payouts/{id}", deps.walletHandler.UpdatePayout)
					r.Post("/advances", deps.walletHandler.CreateCashAdvance)
					r.Get("/{therapist_id}", deps.walletHandler.GetTherapistWallet)
				})
			})

			r.Route("/locations", func(r chi.Router) {
				r.Post("/live", deps.liveLocationHandler.UpdateLocation)
			})

			r.Route("/emergency", func(r chi.Router) {
				r.Post("/trigger", deps.emergencyAlertHandler.TriggerAlert) // public authenticated
				r.Get("/alert/{id}", deps.emergencyAlertHandler.GetAlert)
				r.Post("/alert/{id}/resolve", deps.emergencyAlertHandler.ResolveAlert)

				// Admin-only Dashboard (Consolidated)
				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware(middleware.AdminOperationalRoles, next)
				}).Group(func(r chi.Router) {
					r.Get("/alerts", deps.emergencyAlertHandler.ListAlerts)
					r.Get("/alerts/count", deps.emergencyAlertHandler.CountAlerts)
				})
			})

			// Admin-only Audit Logs (Consolidated from /admin/actions)
			r.Route("/audit-logs", func(r chi.Router) {
				r.Use(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware(middleware.SuperAdminOnlyRoles, next)
				})
				r.Post("/", deps.adminActionHandler.LogAction)
				r.Get("/", deps.adminActionHandler.GetAllActions)
				r.Get("/me", deps.adminActionHandler.GetMyActions)
			})

			// Reports & Accounting (Consolidated from /admin/reports)
			r.Route("/reports", func(r chi.Router) {
				r.Use(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware(middleware.SuperAdminOnlyRoles, next)
				})
				// Accounting
				r.Get("/accounting/summary", deps.reportHandler.GetAccountingSummary)
				r.Get("/accounting/daily", deps.reportHandler.GetDailyAccounting)
				// Ledger
				r.Get("/ledger/summary", deps.reportHandler.GetLedgerSummary)
				r.Get("/ledger/trend", deps.reportHandler.GetLedgerTrend)
				r.Get("/ledger/entries", deps.reportHandler.ListLedgerEntries)
				r.Get("/referrals/summary", deps.reportHandler.GetReferralSummary)
				// Expenses
				r.Route("/expenses", func(r chi.Router) {
					r.Get("/", deps.reportHandler.ListExpenses)
					r.Post("/", deps.reportHandler.CreateExpense)
					r.Post("/upload", deps.reportHandler.UploadExpenseReceipt)
					r.Delete("/{id}", deps.reportHandler.DeleteExpense)
				})
				// Payouts/Settlements (unified: therapists + riders)
				r.Get("/payouts/balances", deps.reportHandler.ListPayoutBalances)
				r.Post("/payouts/settle", deps.reportHandler.RecordSettlement)
				r.Get("/payouts/requests", deps.reportHandler.ListRiderPayoutRequests)
				r.Patch("/payouts/requests/{id}", deps.reportHandler.ResolveRiderPayoutRequest)
				r.Get("/daily-sales", deps.reportHandler.GetDailySalesReport)
				r.Put("/daily-sales/remittances", deps.reportHandler.UpsertDailySalesRemittance)
				r.Get("/daily-sales/export", deps.reportHandler.ExportDailySalesReport)
				r.Get("/booking-export", deps.reportHandler.GetBookingExportReport)
				r.Get("/booking-export/export", deps.reportHandler.ExportBookingReport)
				r.Get("/payroll-adjustments", deps.reportHandler.ListPayrollAdjustments)
				r.Post("/payroll-adjustments", deps.reportHandler.CreatePayrollAdjustment)
				r.Patch("/payroll-adjustments/{id}", deps.reportHandler.UpdatePayrollAdjustment)
				r.Delete("/payroll-adjustments/{id}", deps.reportHandler.DeletePayrollAdjustment)
				r.Get("/therapist-salaries/export", deps.reportHandler.ExportTherapistSalaries)
			})

			// Support Tickets (Consolidated from /admin/support-tickets)
			r.With(func(next http.Handler) http.Handler {
				return middleware.RoleMiddleware(middleware.AdminOperationalRoles, next)
			}).Patch("/support-tickets/{id}/status", deps.ticketHandler.UpdateTicketStatus)

			// Ride Pricing Configuration (Consolidated from /admin/ride-pricing)
			r.Route("/ride-pricing", func(r chi.Router) {
				r.Use(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware(middleware.SuperAdminOnlyRoles, next)
				})
				r.Get("/", deps.adminPricingHandler.GetPricingConfig)
				r.Put("/", deps.adminPricingHandler.UpdatePricingConfig)
			})

			// Additional logic to support /rides/active (handled by RiderHandler) - Moved to inside /rides route block

			r.Route("/messages", func(r chi.Router) {
				r.Post("/conversation", deps.messageHandler.CreateConversation)
				r.Get("/conversations", deps.messageHandler.ListConversations)
				r.Post("/send", deps.messageHandler.SendMessage)
				r.Get("/conversation/{conversation_id}", deps.messageHandler.GetMessages)
				r.Post("/message/{message_id}/read", deps.messageHandler.UpdateMessage) // Shim
				r.Patch("/message/{message_id}", deps.messageHandler.UpdateMessage)

				// Admin-only conversation management
				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware(middleware.AdminOperationalRoles, next)
				}).Group(func(r chi.Router) {
					r.Get("/admin/conversations", deps.messageHandler.ListAllConversations)
					r.Post("/admin/conversations/{conversation_id}/join", deps.messageHandler.AdminJoinConversation)
				})
			})

			r.Route("/referrals", func(r chi.Router) {
				r.Post("/", deps.referralHandler.CreateReferral)
				r.Get("/", deps.referralHandler.ListReferrals)
				r.Get("/summary", deps.referralHandler.GetReferralSummary)
				r.Get("/my-code", deps.referralHandler.GetMyReferralCode)
				r.Get("/code", deps.referralHandler.GetReferralByCode)
				r.Get("/rewards", deps.referralHandler.GetRewards)
				r.Post("/rewards/{reward_id}/redeem", deps.referralHandler.RedeemReward)
			})

			r.Route("/branches", func(r chi.Router) {
				r.Get("/", deps.branchHandler.ListBranches)
				r.Get("/{id}", deps.branchHandler.GetBranch)

				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware(middleware.SuperAdminOnlyRoles, next)
				}).Post("/", deps.branchHandler.CreateBranch)

				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware(middleware.SuperAdminOnlyRoles, next)
				}).Patch("/{id}", deps.branchHandler.UpdateBranch)

				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware(middleware.AdminOperationalRoles, next)
				}).Post("/{id}/deactivate", deps.branchHandler.AdminDeactivateBranch)

				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware(middleware.AdminOperationalRoles, next)
				}).Post("/{id}/reactivate", deps.branchHandler.AdminReactivateBranch)
			})

			// Ride Module Routes
			r.Route("/rides", func(r chi.Router) {
				// Therapist: Request a ride
				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"therapist"}, next)
				}).Post("/request", deps.rideHandler.RequestRide)

				// Rider: Accept/Update rides
				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"rider"}, next)
				}).Patch("/{id}", deps.rideHandler.UpdateRide)

				// Support for /rides/active
				r.Get("/active", deps.riderHandler.GetActiveRide)
			})

			// Rider Module (offers, location, online status)
			r.Route("/rider", func(r chi.Router) {
				r.Use(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"rider"}, next)
				})
				r.Get("/offers", deps.riderHandler.GetPendingOffers)
				r.Get("/active", deps.riderHandler.GetActiveRide)
				r.Get("/rides", deps.riderHandler.GetRideHistory)
				r.Post("/location", deps.riderHandler.UpdateLocation)
				r.Post("/status", deps.riderHandler.UpdateStatus)
				r.Put("/profile", deps.riderHandler.UpdateProfile)
				// Profile creation usually open to auth users or handled separately,
				// but here we put it under rider group for now (or might need to be outside if role check fails)
				r.Post("/profile", deps.riderHandler.CreateProfile)

				// Wallet & Earnings
				r.Get("/wallet", deps.riderWalletHandler.GetWallet)
				r.Get("/transactions", deps.riderWalletHandler.GetTransactions)
				r.Post("/payout", deps.riderWalletHandler.RequestPayout)
				r.Get("/performance", deps.riderWalletHandler.GetPerformance)
				r.Get("/payout-methods", deps.riderWalletHandler.GetPayoutMethods)
				r.Post("/payout-methods", deps.riderWalletHandler.AddPayoutMethod)
				r.Put("/payout-methods/{id}", deps.riderWalletHandler.UpdatePayoutMethod)
				r.Delete("/payout-methods/{id}", deps.riderWalletHandler.DeletePayoutMethod)
				r.Get("/safety-contacts", deps.riderWalletHandler.GetSafetyContacts)
				r.Post("/safety-contacts", deps.riderWalletHandler.AddSafetyContact)
				r.Put("/safety-contacts/{id}", deps.riderWalletHandler.UpdateSafetyContact)
				r.Delete("/safety-contacts/{id}", deps.riderWalletHandler.DeleteSafetyContact)
			})
			r.Route("/therapists", func(r chi.Router) {
				r.Get("/", deps.therapistHandler.ListTherapists)
				r.Get("/{id}", deps.therapistHandler.GetProfile)
				r.Get("/{id}/offers", deps.offersHandler.ListForTherapist)
				r.Get("/{id}/services", deps.therapistHandler.GetServices)
				r.Get("/{id}/documents", deps.therapistHandler.GetDocuments)

				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"therapist"}, next)
				}).Patch("/profile", deps.therapistHandler.UpdateProfile)

				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"therapist"}, next)
				}).Post("/documents", deps.therapistHandler.UploadDocument)

				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"therapist"}, next)
				}).Post("/services", deps.therapistHandler.AddService)

				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"therapist"}, next)
				}).Delete("/services/{service_id}", deps.therapistHandler.RemoveService)

				// Therapist location check-in (marks at_branch=true)
				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware([]string{"therapist"}, next)
				}).Post("/check-in/branch", deps.therapistHandler.CheckInAtBranch)

				// Document verification is a super-admin-only management action.
				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware(middleware.SuperAdminOnlyRoles, next)
				}).Post("/documents/{document_id}/verify", deps.therapistHandler.VerifyDocument)

				// Admins may reassign branch and toggle accept_assignments via profile;
				// the handler strips is_verified for non-super-admins.
				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware(middleware.AdminOperationalRoles, next)
				}).Patch("/{id}/profile", deps.therapistHandler.AdminUpdateProfile)

				// Managing a therapist's services is super-admin only.
				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware(middleware.SuperAdminOnlyRoles, next)
				}).Put("/{id}/services", deps.therapistHandler.AdminUpdateServices)

				// Lifecycle (deactivate/reactivate) is super-admin only.
				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware(middleware.SuperAdminOnlyRoles, next)
				}).Post("/{id}/deactivate", deps.therapistHandler.AdminDeactivateTherapist)

				r.With(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware(middleware.SuperAdminOnlyRoles, next)
				}).Post("/{id}/reactivate", deps.therapistHandler.AdminReactivateTherapist)
			})

			r.Route("/clients", func(r chi.Router) {
				r.Use(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware(middleware.AdminOperationalRoles, next)
				})
				r.Post("/{userID}/deactivate", deps.userHandler.AdminDeactivateClient)
				r.Post("/{userID}/reactivate", deps.userHandler.AdminReactivateClient)
				// Admin-mediated client→therapist blocks
				r.Get("/{userID}/blocked-therapists", deps.userHandler.AdminListClientBlocks)
				r.Post("/{userID}/blocked-therapists", deps.userHandler.AdminBlockTherapistForClient)
				r.Delete("/{userID}/blocked-therapists/{therapistID}", deps.userHandler.AdminUnblockTherapistForClient)
			})

			r.Route("/applications", func(r chi.Router) {
				r.Use(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware(middleware.AdminOperationalRoles, next)
				})
				r.Get("/", deps.applicationHandler.ListAdmin)
				r.Get("/{id}", deps.applicationHandler.GetAdmin)
				r.Patch("/{id}/status", deps.applicationHandler.UpdateStatusAdmin)
			})

			// Global moderation blocks
			r.Route("/moderation", func(r chi.Router) {
				r.Use(func(next http.Handler) http.Handler {
					return middleware.RoleMiddleware(middleware.AdminOperationalRoles, next)
				})
				r.Get("/blocks", deps.moderationHandler.ListBlocks)
				r.Post("/blocks", deps.moderationHandler.BlockUser)
				r.Delete("/blocks/{id}", deps.moderationHandler.UnblockUser)
			})

			// OAuth logout (requires authentication)
			r.Post("/oauth/logout", deps.oauthHandler.OAuthLogout)
		})

		// Other protected routes can go here
	})

}
