package api

import (
	"html/template"
	"log"
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

// Login Pages
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		tmpl := template.Must(template.ParseFiles("web/templates/login.html"))
		tmpl.Execute(w, nil)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	if username == "admin" && password == "admin" {
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

	tmpl := template.Must(template.ParseFiles("web/templates/layout.html", "web/templates/campaigns.html"))
	tmpl.Execute(w, campaigns)
}

func (h *Handler) CreateCampaign(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	start, _ := time.Parse("2025-12-02T15:04", r.FormValue("start_time"))
	end, _ := time.Parse("2025-12-02T15:04", r.FormValue("end_time"))

	// Basic ads parsing from form
	ads := []models.Ad{
		{
			MediaURL:        r.FormValue("media_url"),
			DurationSeconds: 15, // simplified
			CreativeID:      "creative-1",
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
