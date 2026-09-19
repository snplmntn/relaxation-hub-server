// Command seed-service-images is a one-time migration that uploads the landing
// page stock photos to S3 storage and attaches each to its matching service
// (services.preview_image_url).
//
// Services are matched to images by case-insensitive keyword on the service
// name, so it tolerates the difference between the catalog names
// (e.g. "Hagod (Swedish) Massage") and the image filenames
// (e.g. "hiraya - swedish massage.jpg").
//
// Usage (from the relaxation-hub-server directory):
//
//	go run ./cmd/seed-service-images [-images <dir>] [-dry-run] [-force]
//
// By default services that already have a preview_image_url are skipped; pass
// -force to overwrite them. -dry-run reports the matches without uploading.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/snplmntn/relaxation-hub-server/internal/config"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

// imageRule maps a set of name keywords to a stock photo filename. Rules are
// evaluated in order, so more specific keywords ("foot reflex") must precede
// broader ones ("foot").
type imageRule struct {
	keywords []string
	file     string
}

var imageRules = []imageRule{
	{[]string{"ventosa", "baso"}, "hiraya - ventosa massage.jpg"},
	{[]string{"body scrub"}, "hiraya - body scrub.jpg"},
	{[]string{"deep tissue"}, "hiraya - Deep Tissue Massage.jpg"},
	{[]string{"foot reflex", "reflex"}, "hiraya - foot reflex .jpg"},
	{[]string{"foot"}, "hiraya - foot massage.jpg"},
	{[]string{"hilot"}, "hiraya - hilot massage.jpg"},
	{[]string{"hot stone"}, "hiraya - hot stone massage.jpg"},
	{[]string{"natal", "maternal"}, "hiraya - pre and post natal massage.jpg"},
	{[]string{"shiatsu", "pindot"}, "hiraya - shiatsu massage.jpg"},
	{[]string{"sports"}, "hiraya - sports massage.jpg"},
	{[]string{"swedish", "hagod"}, "hiraya - swedish massage.jpg"},
	{[]string{"thai", "hunat"}, "hiraya - thai massage.jpg"},
	{[]string{"combination", "timplado", "signature"}, "hiraya - combination massage.jpg"},
}

func matchImage(serviceName string) (string, bool) {
	name := strings.ToLower(serviceName)
	for _, rule := range imageRules {
		for _, kw := range rule.keywords {
			if strings.Contains(name, kw) {
				return rule.file, true
			}
		}
	}
	return "", false
}

func main() {
	defaultImages := filepath.Join("..", "relaxation-hub-web", "public", "assets", "Stock Photos")
	imagesDir := flag.String("images", defaultImages, "directory containing the stock photo images")
	dryRun := flag.Bool("dry-run", false, "report matches without uploading or updating")
	force := flag.Bool("force", false, "overwrite services that already have a preview image")
	flag.Parse()

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	pool, err := db.InitDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer pool.Close()

	ctx := context.Background()

	var storage service.StorageService
	if !*dryRun {
		s3 := service.NewS3StorageService(ctx, service.S3Config{
			Bucket: cfg.AWSS3Bucket,
			Region: cfg.AWSRegion,
		})
		if !s3.IsConfigured() {
			log.Fatalf("S3 storage is not configured (set AWS_S3_BUCKET and AWS_REGION); use -dry-run to preview matches")
		}
		storage = s3
	}

	rows, err := pool.Query(ctx,
		`SELECT service_id, name, COALESCE(preview_image_url, '')
		 FROM services
		 WHERE deleted_at IS NULL
		 ORDER BY name ASC`)
	if err != nil {
		log.Fatalf("query services: %v", err)
	}

	type svc struct {
		id      int64
		name    string
		preview string
	}
	var services []svc
	for rows.Next() {
		var s svc
		if err := rows.Scan(&s.id, &s.name, &s.preview); err != nil {
			rows.Close()
			log.Fatalf("scan service: %v", err)
		}
		services = append(services, s)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		log.Fatalf("iterate services: %v", err)
	}

	var uploaded, skipped, unmatched int
	usedImages := map[string]bool{}

	for _, s := range services {
		file, ok := matchImage(s.name)
		if !ok {
			fmt.Printf("UNMATCHED  %-45s (no keyword rule)\n", s.name)
			unmatched++
			continue
		}

		if s.preview != "" && !*force {
			fmt.Printf("SKIP       %-45s already has image\n", s.name)
			skipped++
			continue
		}

		path := filepath.Join(*imagesDir, file)
		if _, err := os.Stat(path); err != nil {
			fmt.Printf("MISSING    %-45s image file not found: %s\n", s.name, file)
			unmatched++
			continue
		}
		usedImages[file] = true

		if *dryRun {
			fmt.Printf("WOULD SET  %-45s -> %s\n", s.name, file)
			uploaded++
			continue
		}

		f, err := os.Open(path)
		if err != nil {
			fmt.Printf("ERROR      %-45s open %s: %v\n", s.name, file, err)
			unmatched++
			continue
		}

		contentType := mime.TypeByExtension(filepath.Ext(file))
		if contentType == "" {
			contentType = "image/jpeg"
		}

		key := storage.GenerateKey("services", file)
		url, err := storage.UploadFile(ctx, key, f, contentType)
		f.Close()
		if err != nil {
			fmt.Printf("ERROR      %-45s upload failed: %v\n", s.name, err)
			unmatched++
			continue
		}

		if _, err := pool.Exec(ctx,
			`UPDATE services SET preview_image_url = $1, updated_at = CURRENT_TIMESTAMP
			 WHERE service_id = $2 AND deleted_at IS NULL`,
			url, s.id); err != nil {
			fmt.Printf("ERROR      %-45s db update failed: %v\n", s.name, err)
			unmatched++
			continue
		}
		fmt.Printf("OK         %-45s -> %s\n", s.name, url)
		uploaded++
	}

	fmt.Printf("\nDone. Set/updated: %d, Skipped (existing): %d, Unmatched/errors: %d, Total services: %d\n",
		uploaded, skipped, unmatched, len(services))
}
