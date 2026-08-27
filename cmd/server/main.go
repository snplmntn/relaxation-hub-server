package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	// Embed the IANA timezone database into the binary so time.LoadLocation
	// (e.g. "Asia/Manila") works even on minimal base images like Alpine that
	// ship without the tzdata package.
	_ "time/tzdata"

	"github.com/snplmntn/relaxation-hub-server/internal/app"
	internalConfig "github.com/snplmntn/relaxation-hub-server/internal/config"
)

func main() {
	cfg, err := internalConfig.LoadConfig()
	if err != nil {
		log.Fatal("Error loading .env file: ", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)
	slog.Info("starting relaxation-hub server")

	application, err := app.New(context.Background(), cfg)
	if err != nil {
		log.Fatalf("Failed to initialize server app: %v\n", err)
	}
	application.Start()

	server := &http.Server{
		Addr:    application.Addr(),
		Handler: application.Handler(),
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		printStartupBanner(cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Error starting server: %s\n", err)
		}
	}()

	<-stop
	log.Println("Shutting down gracefully... (press Ctrl+C again to force)")

	ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelShutdown()

	if err := server.Shutdown(ctxShutdown); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	if err := application.Shutdown(ctxShutdown); err != nil {
		log.Printf("Server shutdown completed with worker timeout: %v", err)
	}

	log.Println("Server exited gracefully")
}

func printStartupBanner(port string) {
	fmt.Printf(` _   _ _
| | | (_)
| |_| |_ _ __ __ _ _   _  __ _
|  _  | | '__/ _`+"`"+` | | | |/ _`+"`"+` |
| | | | | | | (_| | |_| | (_| |
\_| |_/_|_|  \__,_|\__, |\__,_|
                    __/ |
                   |___/
Server started on port: %s...
`, port)
}
