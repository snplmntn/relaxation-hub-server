package app

import (
	"context"
	"log"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/snplmntn/relaxation-hub-server/internal/broadcaster"
	"github.com/snplmntn/relaxation-hub-server/internal/config"
	"github.com/snplmntn/relaxation-hub-server/internal/handler"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/oauth"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
	ws "github.com/snplmntn/relaxation-hub-server/internal/websocket"
)

type dependencies struct {
	cfg                            *config.Config
	ticketLimiter                  *middleware.RateLimiter
	authHandler                    *handler.AuthHandler
	addressHandler                 *handler.AddressHandler
	bookingHandler                 *handler.BookingHandler
	paymentHandler                 *handler.PaymentHandler
	promotionHandler               *handler.PromotionHandler
	reviewHandler                  *handler.ReviewHandler
	liveLocationHandler            *handler.LiveLocationHandler
	emergencyAlertHandler          *handler.EmergencyAlertHandler
	messageHandler                 *handler.MessageHandler
	referralHandler                *handler.ReferralHandler
	branchHandler                  *handler.BranchHandler
	partnerHotelHandler            *handler.PartnerHotelHandler
	applicationHandler             *handler.ApplicationHandler
	therapistHandler               *handler.TherapistHandler
	dayViewOrderHandler            *handler.DayViewOrderHandler
	offersHandler                  *handler.OffersHandler
	ticketHandler                  *handler.SupportTicketHandler
	legalDocHandler                *handler.LegalDocumentHandler
	riderWalletHandler             *handler.RiderWalletHandler
	rideHandler                    *handler.RideHandler
	riderHandler                   *handler.RiderHandler
	adminPricingHandler            *handler.AdminPricingHandler
	locationHandler                *handler.LocationHandler
	availabilityHandler            *handler.AvailabilityHandler
	accountSecurityHandler         *handler.AccountSecurityHandler
	userHandler                    *handler.UserHandler
	moderationHandler              *handler.ModerationHandler
	adminActionHandler             *handler.AdminActionHandler
	serviceHandler                 *handler.ServiceHandler
	wsHandler                      *handler.WebSocketHandler
	reportHandler                  *handler.ReportHandler
	reportDependencyStatusProvider *handler.ReportDependencyStatusProvider
	productHandler                 *handler.ProductHandler
	bookingGroupHandler            *handler.BookingGroupHandler
	bookingCheckoutHandler         *handler.BookingCheckoutHandler
	recurringBookingHandler        *handler.RecurringBookingHandler
	cartHandler                    *handler.CartHandler
	oauthHandler                   *handler.OAuthHandler
	googleAuthHandler              *handler.GoogleAuthHandler
	authLimiter                    *middleware.RateLimiter
	configHandler                  *handler.ConfigHandler
	walletHandler                  *handler.WalletHandler
	notificationHandler            *handler.NotificationHandler
	staffAttendanceHandler         *handler.StaffAttendanceHandler
	payrollHandler                 *handler.PayrollHandler
	cashRemittanceHandler          *handler.CashRemittanceHandler
	accountingHandler              *handler.AccountingHandler
	blogPostHandler                *handler.BlogPostHandler
	landingSettingsHandler         *handler.LandingSettingsHandler
	userRepo                       repository.UserRepository
}

func buildDependencies(ctx context.Context, cfg *config.Config, pool *pgxpool.Pool, hub *ws.Hub, workers *WorkerManager) (*dependencies, error) {
	storageService := service.NewS3StorageService(ctx, service.S3Config{
		Bucket: cfg.AWSS3Bucket,
		Region: cfg.AWSRegion,
	})

	broadcaster.SetHub(hub)

	mapboxToken := os.Getenv("MAPBOX_API_TOKEN")
	geocoderProvider := strings.ToLower(os.Getenv("GEOCODER_PROVIDER"))
	routingProvider := strings.ToLower(os.Getenv("ROUTING_PROVIDER"))

	var baseGeocoder service.Geocoder
	switch geocoderProvider {
	case "nominatim":
		baseGeocoder = service.NewNominatimGeocoder(os.Getenv("NOMINATIM_BASE"), os.Getenv("NOMINATIM_USER_AGENT"))
	default:
		if mapboxToken == "" {
			slog.Warn("MAPBOX_API_TOKEN not set; falling back to Nominatim geocoder")
			baseGeocoder = service.NewNominatimGeocoder(os.Getenv("NOMINATIM_BASE"), os.Getenv("NOMINATIM_USER_AGENT"))
		} else {
			baseGeocoder = service.NewMapboxGeocoder(mapboxToken)
		}
	}

	geocoder, err := service.NewCachedGeocoder(baseGeocoder, 1000, 24*time.Hour)
	if err != nil {
		slog.Error("failed to create cached geocoder", "error", err)
		geocoder = baseGeocoder
	}

	userRepo := repository.NewUserRepository(pool)
	googleAuthRepo := repository.NewGoogleAuthRepository(pool)
	accountSecurityRepo := repository.NewAccountSecurityRepository(pool)
	moderationRepo := repository.NewModerationRepository(pool)
	broadcaster.SetUserRepo(userRepo)
	authService := service.NewAuthService(userRepo, cfg)
	googleAuthService := service.NewGoogleAuthService(
		googleAuthRepo,
		oauth.NewGoogleCredentialVerifier(cfg.GoogleOAuthClientID),
		cfg.JWTKey,
	)
	accountSecurityService := service.NewAccountSecurityService(accountSecurityRepo)
	accountSecurityHandler := handler.NewAccountSecurityHandler(accountSecurityService)
	rateLimiter := middleware.NewRateLimiter(workers.Context(), pool, middleware.DefaultRateLimitConfig())
	googleAuthLimiter := middleware.NewRateLimiter(workers.Context(), pool, middleware.RateLimitConfig{
		MaxAttempts:     20,
		LockoutDuration: 15 * time.Minute,
		ResetWindow:     15 * time.Minute,
		CheckInterval:   time.Minute,
	})
	ticketLimiter := middleware.NewRateLimiter(workers.Context(), pool, middleware.RateLimitConfig{
		MaxAttempts:     2,
		LockoutDuration: 10 * time.Minute,
		ResetWindow:     10 * time.Minute,
		CheckInterval:   time.Minute,
	})
	referralRepo := repository.NewReferralRepository(pool)
	referralService := service.NewReferralService(referralRepo)
	authHandler := handler.NewAuthHandler(authService, rateLimiter, referralService)
	addressRepo := repository.NewAddressRepository(pool)
	addressService := service.NewAddressService(addressRepo, nil)
	addressService.SetGeocoder(geocoder)
	addressHandler := handler.NewAddressHandler(addressService)
	bookingRepo := repository.NewBookingRepository(pool)
	bookingReferralRepo := repository.NewBookingReferralRepository(pool)
	therapistRepo := repository.NewTherapistRepository(pool)
	promotionRepo := repository.NewPromotionRepository(pool)
	branchRepo := repository.NewBranchRepository(pool)
	assignmentQueueRepo := repository.NewDisabledAssignmentQueueRepository()
	var offerRepo repository.BookingOfferRepository
	if cfg.AutomatedOffersEnabled {
		assignmentQueueRepo = repository.NewAssignmentQueueRepository(pool)
		offerRepo = repository.NewBookingOfferRepository(pool)
	}
	serviceRepo := repository.NewServiceRepository(pool)
	ticketRepo := repository.NewSupportTicketRepository(pool)
	legalDocRepo := repository.NewLegalDocumentRepository(pool)

	fcmService, err := service.NewFCMService(ctx)
	if err != nil {
		log.Printf("Warning: FCM service initialization failed: %v (push notifications will be disabled)", err)
	}

	notificationRepo := repository.NewNotificationRepository(pool)
	notificationService := service.NewNotificationService(notificationRepo, userRepo, fcmService)
	notificationHandler := handler.NewNotificationHandler(notificationService)

	emailLocation, err := time.LoadLocation(cfg.BookingEmailTimezone)
	if err != nil {
		slog.Warn("invalid BOOKING_EMAIL_TIMEZONE; falling back to Asia/Manila", "timezone", cfg.BookingEmailTimezone, "error", err)
		emailLocation = time.FixedZone("Asia/Manila", 8*60*60)
	}
	var bookingEmailService *service.BookingEmailService
	smtpSender := service.NewSMTPEmailSender(cfg.SMTP)
	if smtpSender.IsConfigured() {
		bookingEmailService = service.NewBookingEmailService(bookingRepo, userRepo, smtpSender, emailLocation)
	} else {
		slog.Warn("SMTP email sender is not configured; booking emails are disabled")
	}

	messageRepo := repository.NewMessageRepository(pool)
	messageService := service.NewMessageService(messageRepo, notificationService, userRepo, hub)

	walletRepo := repository.NewWalletRepository(pool)
	walletService := service.NewWalletService(pool, walletRepo, bookingRepo)
	walletHandler := handler.NewWalletHandler(walletService)

	rideRepo := repository.NewRideRepository(pool)
	var rideOfferRepo repository.RideOfferRepository
	if cfg.AutomatedOffersEnabled {
		rideOfferRepo = repository.NewRideOfferRepository(pool)
	}
	ridePricingService := service.NewRidePricingService(pool)
	rideMatchingService := service.NewRideMatchingService(pool)
	rideService := service.NewRideService(rideRepo, rideOfferRepo, ridePricingService, rideMatchingService, pool)
	rideService.SetNotificationService(notificationService)
	rideService.SetMessageService(messageService)
	rideService.SetGeocoder(geocoder)
	rideHandler := handler.NewRideHandler(rideService)
	riderHandler := handler.NewRiderHandler(rideService)
	adminPricingHandler := handler.NewAdminPricingHandler(ridePricingService)

	serviceAreaRepo := repository.NewServiceAreaRepository(pool)
	locationService := service.NewLocationService(serviceAreaRepo)
	adminActionRepo := repository.NewAdminActionRepository(pool)
	locationHandler := handler.NewLocationHandler(locationService, adminActionRepo)

	extensionRequestRepo := repository.NewExtensionRequestRepository(pool)
	bookingService := service.NewBookingService(bookingRepo, promotionRepo, pool, assignmentQueueRepo, therapistRepo, offerRepo, serviceRepo, addressRepo, userRepo, messageService, notificationService, extensionRequestRepo, walletService, rideService, locationService)
	bookingService.SetBookingReferralRepository(bookingReferralRepo)
	bookingService.SetBookingEmailService(bookingEmailService)
	bookingServiceRepo := repository.NewBookingServiceRepository(pool)
	bookingService.SetBookingServiceRepository(bookingServiceRepo)
	rideService.SetBookingUpdater(bookingService)
	paymentRepo := repository.NewPaymentRepository(pool)
	paymentService := service.NewPaymentService(paymentRepo)
	bookingHandler := handler.NewBookingHandler(bookingService, paymentService, serviceRepo, addressRepo, therapistRepo, storageService)
	paymentHandler := handler.NewPaymentHandler(paymentService, bookingRepo, serviceRepo, addressRepo)
	promotionService := service.NewPromotionService(promotionRepo, userRepo)
	promotionHandler := handler.NewPromotionHandler(promotionService)
	reviewRepo := repository.NewReviewRepository(pool)
	reviewService := service.NewReviewService(reviewRepo, notificationService, userRepo)
	clientReviewRepo := repository.NewClientReviewRepository(pool)
	clientReviewService := service.NewClientReviewService(clientReviewRepo)
	reviewHandler := handler.NewReviewHandler(reviewService, clientReviewService, bookingRepo, serviceRepo, userRepo)
	liveLocationRepo := repository.NewLiveLocationRepository(pool)
	liveLocationService := service.NewLiveLocationService(liveLocationRepo, bookingRepo, hub)
	liveLocationHandler := handler.NewLiveLocationHandler(liveLocationService)
	emergencyAlertRepo := repository.NewEmergencyAlertRepository(pool)
	emergencyAlertService := service.NewEmergencyAlertService(emergencyAlertRepo)
	emergencyAlertHandler := handler.NewEmergencyAlertHandler(emergencyAlertService, bookingService)
	messageHandler := handler.NewMessageHandler(messageService)
	referralHandler := handler.NewReferralHandler(referralService)
	branchService := service.NewBranchService(branchRepo)
	branchHandler := handler.NewBranchHandler(branchService)
	partnerHotelRepo := repository.NewPartnerHotelRepository(pool)
	partnerHotelService := service.NewPartnerHotelService(partnerHotelRepo)
	partnerHotelHandler := handler.NewPartnerHotelHandler(partnerHotelService)
	applicationRepo := repository.NewApplicationRepository(pool)
	applicationService := service.NewApplicationService(applicationRepo, authService, userRepo, branchRepo, therapistRepo, rideRepo)
	applicationHandler := handler.NewApplicationHandler(applicationService)
	bookingLifecycleRepo, _ := bookingRepo.(interface {
		HasActiveNonFinalBookings(ctx context.Context, therapistID int64) (bool, error)
	})
	therapistService := service.NewTherapistService(therapistRepo, userRepo, bookingLifecycleRepo)
	therapistHandler := handler.NewTherapistHandler(therapistService, storageService)
	dayViewOrderRepo := repository.NewDayViewOrderRepository(pool)
	dayViewOrderService := service.NewDayViewOrderService(dayViewOrderRepo)
	dayViewOrderHandler := handler.NewDayViewOrderHandler(dayViewOrderService)
	offersHandler := handler.NewOffersHandler(bookingService)
	ticketService := service.NewSupportTicketService(ticketRepo, userRepo)
	ticketHandler := handler.NewSupportTicketHandler(ticketService, storageService)
	legalDocService := service.NewLegalDocumentService(legalDocRepo)
	legalDocHandler := handler.NewLegalDocumentHandler(legalDocService)

	riderWalletService := service.NewRiderWalletService(pool)
	riderWalletHandler := handler.NewRiderWalletHandler(riderWalletService)

	logisticsService := service.NewLogisticsService(rideService, bookingRepo, therapistRepo, addressRepo, pool)
	if !cfg.AutomatedOffersEnabled {
		logisticsService.DisableAutomaticDispatch()
	}
	bookingService.SetLogisticsService(logisticsService)

	authHandler.SetRideRepository(rideRepo)
	authHandler.SetRiderWalletService(riderWalletService)

	opsNotifier := func(ctx context.Context, subject string, details map[string]string) error {
		log.Printf("OPS ALERT: %s - %v", subject, details)

		go func() {
			bgCtx := context.Background()

			if userRepo != nil && notificationService != nil {
				if err := sendOpsAdminNotifications(bgCtx, func(ctx context.Context) ([]model.User, error) {
					return userRepo.ListUsers(ctx, "admin")
				}, notificationService.CreateMany, subject, details); err != nil {
					log.Printf("opsNotifier: failed to create admin notifications: %v", err)
					return
				}
			}
		}()

		return nil
	}

	therapistMatchingService := service.NewTherapistMatchingService(therapistRepo, bookingRepo)
	bookingAvailabilityService := service.NewBookingAvailabilityService(therapistMatchingService, addressRepo)
	availabilityHandler := handler.NewAvailabilityHandler(therapistMatchingService, bookingAvailabilityService)
	if cfg.AutomatedOffersEnabled {
		assignmentWorker := service.NewAssignmentWorker(pool, assignmentQueueRepo, bookingRepo, paymentRepo, offerRepo, serviceRepo, serviceAreaRepo, therapistRepo, therapistMatchingService, notificationService, opsNotifier)
		workers.Add("assignment", assignmentWorker, assignmentWorker)
	} else {
		slog.Info("automated therapist and rider offers disabled; assignments are manual")
	}

	ledgerRepo := repository.NewLedgerRepository(pool)
	completionWorker := service.NewCompletionWorker(pool, bookingRepo, paymentRepo, serviceRepo, ledgerRepo, walletService, notificationService)
	completionWorker.SetBookingEmailService(bookingEmailService)
	completionWorker.SetBookingServiceRepository(bookingServiceRepo)
	workers.Add("completion", completionWorker, completionWorker)

	reminderJobRepo := bookingRepo.(interface {
		ClaimDueReminderJobs(ctx context.Context, now time.Time, limit int) ([]repository.BookingReminderJob, error)
		MarkReminderJobProcessed(ctx context.Context, jobID int64) error
		InsertEvent(ctx context.Context, bookingID int64, eventType string, actorID *int64, metadata map[string]any) error
		ListUpcomingBookingsForReminder(ctx context.Context, start, end time.Time, eventTypeExclude string) ([]model.Booking, error)
	})
	upcomingBookingWorker := service.NewUpcomingBookingWorker(reminderJobRepo, notificationService)
	upcomingBookingWorker.SetBookingEmailService(bookingEmailService, emailLocation, cfg.BookingDDayEmailHour)
	workers.Add("upcoming", upcomingBookingWorker, upcomingBookingWorker)

	if cfg.AutomatedOffersEnabled {
		var routingService service.RoutingService
		switch routingProvider {
		case "osrm":
			routingService = service.NewOSRMRoutingService(os.Getenv("OSRM_BASE"), os.Getenv("OSRM_PROFILE"))
		default:
			if mapboxToken == "" {
				slog.Warn("MAPBOX_API_TOKEN not set; falling back to OSRM routing")
				routingService = service.NewOSRMRoutingService(os.Getenv("OSRM_BASE"), os.Getenv("OSRM_PROFILE"))
			} else {
				routingService = service.NewMapboxRoutingService(mapboxToken)
			}
		}

		riderDispatchWorker := service.NewRiderDispatchWorker(bookingRepo.(service.RiderDispatchBookingRepository), rideService, routingService, pool)
		workers.Add("rider_dispatch", riderDispatchWorker, riderDispatchWorker)
	}

	userService := service.NewUserService(userRepo, addressRepo, rideRepo)
	userHandler := handler.NewUserHandler(userService, storageService, authService)
	moderationService := service.NewModerationService(moderationRepo)
	moderationHandler := handler.NewModerationHandler(moderationService)
	adminActionService := service.NewAdminActionService(adminActionRepo)
	adminActionHandler := handler.NewAdminActionHandler(adminActionService)
	serviceCache := service.NewServiceCache()
	serviceCatalog := service.NewServiceCatalog(serviceRepo, serviceCache)
	serviceHandler := handler.NewServiceHandler(serviceCatalog, storageService)
	wsHandler := handler.NewWebSocketHandler(hub, cfg.JWTKey)
	reportExportRepo := repository.NewReportExportRepository(pool)
	reportExportService := service.NewReportExportService(reportExportRepo)
	reportHandler := handler.NewReportHandler(bookingRepo, ledgerRepo, storageService, riderWalletService)
	reportHandler.SetBookingReferralRepository(bookingReferralRepo)
	reportHandler.SetReportExportService(reportExportService)
	reportDependencyStatusProvider := handler.NewReportDependencyStatusProvider(reportHandler, pool.Ping)
	reportHandler.SetDependencyStatusProvider(reportDependencyStatusProvider)
	_ = reportDependencyStatusProvider.Snapshot(context.Background())

	staffAttendanceRepo := repository.NewStaffAttendanceRepository(pool)
	staffAttendanceService := service.NewStaffAttendanceService(staffAttendanceRepo)
	staffAttendanceHandler := handler.NewStaffAttendanceHandler(staffAttendanceService)
	payrollRepo := repository.NewPayrollRepository(pool)
	payrollService := service.NewPayrollService(payrollRepo)
	payrollHandler := handler.NewPayrollHandler(payrollService)

	cashRemittanceRepo := repository.NewCashRemittanceRepository(pool)
	cashRemittanceService := service.NewCashRemittanceService(cashRemittanceRepo)
	cashRemittanceHandler := handler.NewCashRemittanceHandler(cashRemittanceService)

	accountingRepo := repository.NewAccountingRepository(pool)
	accountingService := service.NewAccountingService(accountingRepo)
	accountingHandler := handler.NewAccountingHandler(accountingService)

	productRepo := repository.NewProductRepository(pool)
	productCatalog := service.NewProductCatalog(productRepo, storageService)
	productHandler := handler.NewProductHandler(productCatalog, storageService)
	blogPostRepo := repository.NewBlogPostRepository(pool)
	blogPostService := service.NewBlogPostService(blogPostRepo)
	blogPostHandler := handler.NewBlogPostHandler(blogPostService, storageService)
	landingSettingsRepo := repository.NewLandingSettingsRepository(pool)
	landingSettingsService := service.NewLandingSettingsService(landingSettingsRepo)
	landingSettingsHandler := handler.NewLandingSettingsHandler(landingSettingsService)
	bookingGroupRepo := repository.NewBookingGroupRepository(pool)
	bookingAddonRepo := repository.NewBookingAddonRepository(pool)
	bookingGroupService := service.NewBookingGroupService(pool, bookingGroupRepo, bookingRepo, bookingAddonRepo, productRepo, serviceRepo, assignmentQueueRepo, addressRepo, locationService, branchRepo, promotionRepo, userRepo)
	bookingGroupService.SetBookingServiceRepository(bookingServiceRepo)
	bookingGroupService.SetTherapistRepository(therapistRepo)
	bookingGroupHandler := handler.NewBookingGroupHandler(bookingGroupService, productRepo)

	// Online payment. Absent PayMongo credentials the handler stays nil and the
	// routes answer 503, so an unconfigured environment simply does not offer
	// the option rather than failing at startup.
	var bookingCheckoutHandler *handler.BookingCheckoutHandler
	if cfg.PayMongo.Enabled() {
		paymongoClient := service.NewPayMongoClient(cfg.PayMongo.SecretKey, cfg.PayMongo.WebhookSecret, cfg.PayMongo.LiveMode)
		bookingCheckoutService := service.NewBookingCheckoutService(
			repository.NewBookingCheckoutRepository(pool),
			paymentRepo, bookingRepo, userRepo,
			bookingService, bookingGroupService, paymongoClient,
			cfg.PayMongo.SuccessURL, cfg.PayMongo.CancelURL,
		)
		bookingCheckoutHandler = handler.NewBookingCheckoutHandler(bookingCheckoutService, paymongoClient)
		slog.Info("[Startup] online payment enabled", "live_mode", cfg.PayMongo.LiveMode)
	} else {
		slog.Info("[Startup] online payment disabled (PAYMONGO_SECRET_KEY not set)")
	}

	recurringBookingRepo := repository.NewRecurringBookingRepository(pool)
	recurringBookingService := service.NewRecurringBookingService(pool, recurringBookingRepo, bookingRepo, serviceRepo, assignmentQueueRepo, userRepo)
	recurringBookingHandler := handler.NewRecurringBookingHandler(recurringBookingService)
	recurringBookingWorker := service.NewRecurringBookingWorker(recurringBookingRepo, recurringBookingService)
	workers.Add("recurring_booking", recurringBookingWorker, recurringBookingWorker)

	cartRepo := repository.NewCartRepository(pool)
	cartHandler := handler.NewCartHandler(cartRepo)

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

	return &dependencies{
		cfg:                            cfg,
		ticketLimiter:                  ticketLimiter,
		authHandler:                    authHandler,
		addressHandler:                 addressHandler,
		bookingHandler:                 bookingHandler,
		paymentHandler:                 paymentHandler,
		promotionHandler:               promotionHandler,
		reviewHandler:                  reviewHandler,
		liveLocationHandler:            liveLocationHandler,
		emergencyAlertHandler:          emergencyAlertHandler,
		messageHandler:                 messageHandler,
		referralHandler:                referralHandler,
		branchHandler:                  branchHandler,
		partnerHotelHandler:            partnerHotelHandler,
		applicationHandler:             applicationHandler,
		therapistHandler:               therapistHandler,
		dayViewOrderHandler:            dayViewOrderHandler,
		offersHandler:                  offersHandler,
		ticketHandler:                  ticketHandler,
		legalDocHandler:                legalDocHandler,
		riderWalletHandler:             riderWalletHandler,
		rideHandler:                    rideHandler,
		riderHandler:                   riderHandler,
		adminPricingHandler:            adminPricingHandler,
		locationHandler:                locationHandler,
		availabilityHandler:            availabilityHandler,
		accountSecurityHandler:         accountSecurityHandler,
		userHandler:                    userHandler,
		moderationHandler:              moderationHandler,
		adminActionHandler:             adminActionHandler,
		serviceHandler:                 serviceHandler,
		wsHandler:                      wsHandler,
		reportHandler:                  reportHandler,
		reportDependencyStatusProvider: reportDependencyStatusProvider,
		productHandler:                 productHandler,
		bookingGroupHandler:            bookingGroupHandler,
		bookingCheckoutHandler:         bookingCheckoutHandler,
		recurringBookingHandler:        recurringBookingHandler,
		cartHandler:                    cartHandler,
		oauthHandler:                   handler.NewOAuthHandler(userRepo, cfg.JWTKey, 24*time.Hour),
		googleAuthHandler:              handler.NewGoogleAuthHandler(googleAuthService),
		authLimiter:                    googleAuthLimiter,
		configHandler:                  handler.NewConfigHandler(),
		walletHandler:                  walletHandler,
		notificationHandler:            notificationHandler,
		staffAttendanceHandler:         staffAttendanceHandler,
		payrollHandler:                 payrollHandler,
		cashRemittanceHandler:          cashRemittanceHandler,
		accountingHandler:              accountingHandler,
		blogPostHandler:                blogPostHandler,
		landingSettingsHandler:         landingSettingsHandler,
		userRepo:                       userRepo,
	}, nil
}

func sendOpsAdminNotifications(ctx context.Context, listAdmins func(context.Context) ([]model.User, error), createMany func(context.Context, []*model.CreateNotificationRequest) ([]*model.Notification, error), subject string, details map[string]string) error {
	admins, err := listAdmins(ctx)
	if err != nil {
		return err
	}
	if len(admins) == 0 {
		return nil
	}

	msg := subject
	if len(details) > 0 {
		for k, v := range details {
			msg = msg + "; " + k + "=" + v
		}
	}

	reqs := make([]*model.CreateNotificationRequest, 0, len(admins))
	for _, admin := range admins {
		reqs = append(reqs, &model.CreateNotificationRequest{
			UserID:  int64(admin.UserID),
			Type:    "ops_alert",
			Title:   "System Alert: " + subject,
			Message: msg,
		})
	}

	_, err = createMany(ctx, reqs)
	return err
}
