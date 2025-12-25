package api

import (
	"html/template"
	"log"
	"strings"
	"net/http"
	"rockbot-adserver/internal/models"
	"rockbot-adserver/internal/service"
	"time"
)

type Handler struct {
	service *service.AdService
}

func NewHandler(s *service.AdService) *Handler {
	return &Handler{service: s}
}

// Middleware for Auth
func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session_token")
		if err != nil || cookie.Value != "valid-token" {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}

// Login Page
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		tmpl := template.Must(template.ParseFiles("web/templates/login.html"))
		tmpl.Execute(w, nil)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	// TODO: Implement proper authentication. For now we're using a simple hardcoded username and password.
	if strings.ToLower(username) == "admin" && strings.ToLower(password) == "admin" {
		http.SetCookie(w, &http.Cookie{
			Name:  "session_token",
			Value: "valid-token",
			Path:  "/",
		})
		http.Redirect(w, r, "/campaigns", http.StatusSeeOther)
		return
	}
	http.Error(w, "Invalid Credentials", http.StatusUnauthorized)
}

// Campaign UI
func (h *Handler) ListCampaigns(w http.ResponseWriter, r *http.Request) {
	campaigns, err := h.service.ListCampaigns()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	availableAds, err := h.service.GetAvailableAds()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := struct {
		Campaigns    []models.Campaign
		AvailableAds []models.Ad
	}{
		Campaigns:    campaigns,
		AvailableAds: availableAds,
	}

	tmpl := template.Must(template.ParseFiles("web/templates/layout.html", "web/templates/campaigns.html"))
	tmpl.Execute(w, data)
}

func (h *Handler) CreateCampaign(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	start, _ := time.Parse("2006-01-02T15:04", r.FormValue("start_time"))
	end, _ := time.Parse("2006-01-02T15:04", r.FormValue("end_time"))

	// Get the selected available ad by media_url
	mediaURL := r.FormValue("media_url")
	availableAd, err := h.service.GetAvailableAdByMediaURL(mediaURL)
	if err != nil {
		http.Error(w, "Invalid media URL selected", http.StatusBadRequest)
		return
	}

	// Create a new ad linked to the campaign (copying properties from available ad)
	ads := []models.Ad{
		{
			MediaURL:        availableAd.MediaURL,
			DurationSeconds: availableAd.DurationSeconds,
			CreativeID:      availableAd.CreativeID,
		},
	}
	campaign := models.Campaign{
		Name:      r.FormValue("name"),
		StartTime: start,
		EndTime:   end,
		TargetDMA: r.FormValue("target_dma"),
		Ads:       ads,
	}

	if err := h.service.CreateCampaign(campaign); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/campaigns", http.StatusSeeOther)
}

// Client Demo
func (h *Handler) ClientDemo(w http.ResponseWriter, r *http.Request) {
	log.Println("Inside ClientDemo function")
	tmpl := template.Must(template.ParseFiles("web/templates/layout.html", "web/templates/client_demo.html"))
	log.Println("Inside ClientDemo function", tmpl.Tree.Root.Nodes)
	tmpl.Execute(w, nil)
}

// API: Serve Ads
func (h *Handler) ServeAds(w http.ResponseWriter, r *http.Request) {
	dma := r.URL.Query().Get("dma")
	clientID := r.URL.Query().Get("client_id")

	if clientID == "" {
		http.Error(w, "Missing client_id", http.StatusBadRequest)
		return
	}

	xmlResponse, err := h.service.GetAdsForClient(clientID, dma)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/xml")
	w.Write([]byte(xmlResponse))
}
