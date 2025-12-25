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
	h := api.NewHandler(svc)

	// Routes
	http.HandleFunc("/login", h.Login)

	// Protected UI Routes
	http.HandleFunc("/", api.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/campaigns", http.StatusSeeOther)
	}))
	http.HandleFunc("/campaigns", api.AuthMiddleware(h.ListCampaigns))
	http.HandleFunc("/campaigns/create", api.AuthMiddleware(h.CreateCampaign))
	http.HandleFunc("/client", api.AuthMiddleware(h.ClientDemo))

	// Public API
	http.HandleFunc("/vast", h.ServeAds)

	log.Println("Server starting on :8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
