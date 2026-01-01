package main

import (
	"log"
	"net/http"
	"rockbot-adserver/internal/api"
	"rockbot-adserver/internal/models"
	"rockbot-adserver/internal/service"
	"rockbot-adserver/internal/store"

	"github.com/google/uuid"
)

func main() {
	// Initialize Store
	db, err := store.NewStore("adserver.db")
	if err != nil {
		log.Fatalf("Failed to init store: %v", err)
	}

	// Seed available ads on startup
	availableAds := []models.Ad{
		{
			ID:              uuid.New().String(),
			MediaURL:        "http://commondatastorage.googleapis.com/gtv-videos-bucket/sample/ForBiggerBlazes.mp4",
			DurationSeconds: 15,
			CreativeID:      "creative-1",
		},
		{
			ID:              uuid.New().String(),
			MediaURL:        "http://commondatastorage.googleapis.com/gtv-videos-bucket/sample/ForBiggerEscapes.mp4",
			DurationSeconds: 15,
			CreativeID:      "creative-2",
		},
		{
			ID:              uuid.New().String(),
			MediaURL:        "http://commondatastorage.googleapis.com/gtv-videos-bucket/sample/ForBiggerFun.mp4",
			DurationSeconds: 15,
			CreativeID:      "creative-3",
		},
	}
	if err := db.SeedAvailableAds(availableAds); err != nil {
		log.Fatalf("Failed to seed available ads: %v", err)
	}

	// Initialize Service
	svc := service.NewAdService(db)

	// Initialize Handlers
	h := api.NewHandler(svc, db)

	// Create logging middleware
	loggingMiddleware := api.LoggingMiddleware(db)

	// Routes with logging middleware
	http.Handle("/login", loggingMiddleware(http.HandlerFunc(h.Login)))

	// Protected UI Routes
	http.Handle("/", loggingMiddleware(api.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/campaigns", http.StatusSeeOther)
	})))
	http.Handle("/campaigns", loggingMiddleware(api.AuthMiddleware(h.ListCampaigns)))
	http.Handle("/campaigns/create", loggingMiddleware(api.AuthMiddleware(h.CreateCampaign)))
	http.Handle("/client", loggingMiddleware(api.AuthMiddleware(h.ClientDemo)))
	http.Handle("/logs", loggingMiddleware(api.AuthMiddleware(h.ListRequestLogs)))

	// Request logs API (protected)
	http.Handle("/api/logs", loggingMiddleware(api.AuthMiddleware(h.QueryRequestLogs)))

	// Public API
	http.Handle("/vast", loggingMiddleware(http.HandlerFunc(h.ServeAds)))

	log.Println("Server starting on :8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
