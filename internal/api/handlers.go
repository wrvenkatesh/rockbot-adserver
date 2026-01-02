package api

import (
	"bytes"
	"encoding/json"
	"html/template"
	"io"
	"log"
	"net/http"
	"rockbot-adserver/internal/models"
	"rockbot-adserver/internal/service"
	"rockbot-adserver/internal/store"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Handler struct {
	service *service.AdService
	store   *store.Store
}

func NewHandler(s *service.AdService, st *store.Store) *Handler {
	return &Handler{service: s, store: st}
}

// responseWriter wraps http.ResponseWriter to capture response body and status
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
		body:           &bytes.Buffer{},
	}
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	rw.body.Write(b)
	return rw.ResponseWriter.Write(b)
}

// LoggingMiddleware captures and logs all requests and responses
func LoggingMiddleware(store *store.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startTime := time.Now()

			// Capture request body
			var requestBodyBytes []byte
			if r.Body != nil {
				requestBodyBytes, _ = io.ReadAll(r.Body)
				r.Body = io.NopCloser(bytes.NewBuffer(requestBodyBytes))
			}

			// Capture request headers
			requestHeadersBytes, _ := json.Marshal(r.Header)

			// Capture query parameters
			queryParams := r.URL.RawQuery

			// Wrap response writer to capture response
			rw := newResponseWriter(w)

			// Process request
			next.ServeHTTP(rw, r)

			// Calculate duration
			duration := time.Since(startTime)

			// Capture response body (limit size to avoid storing huge responses)
			responseBody := rw.body.String()
			maxBodySize := 10000 // 10KB limit
			if len(responseBody) > maxBodySize {
				responseBody = responseBody[:maxBodySize] + "... [truncated]"
			}

			// Capture response headers
			responseHeadersBytes, _ := json.Marshal(rw.Header())

			// Limit request body size as well
			requestBody := string(requestBodyBytes)
			if len(requestBody) > maxBodySize {
				requestBody = requestBody[:maxBodySize] + "... [truncated]"
			}

			// Create request log
			requestLog := models.RequestLog{
				ID:              uuid.New().String(),
				Method:          r.Method,
				Path:            r.URL.Path,
				QueryParams:     queryParams,
				RequestHeaders:  string(requestHeadersBytes),
				RequestBody:     requestBody,
				ResponseStatus:  rw.statusCode,
				ResponseHeaders: string(responseHeadersBytes),
				ResponseBody:    responseBody,
				DurationMs:      duration.Milliseconds(),
				Timestamp:       startTime,
				RemoteAddr:      r.RemoteAddr,
				UserAgent:       r.UserAgent(),
			}

			// Save log asynchronously to avoid blocking the response
			go func() {
				if err := store.SaveRequestLog(requestLog); err != nil {
					log.Printf("Failed to save request log: %v", err)
				}
			}()
		})
	}
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
		Campaign        *models.Campaign
		Campaigns       []models.Campaign
		AvailableAds    []models.Ad
		CurrentMediaURL string
	}{
		Campaign:        nil,
		Campaigns:       campaigns,
		AvailableAds:    availableAds,
		CurrentMediaURL: "",
	}

	log.Println("data.Campaigns", data.Campaigns)
	tmpl := template.Must(template.ParseFiles("web/templates/layout.html", "web/templates/campaigns.html"))
	tmpl.Execute(w, data)
}

func (h *Handler) CreateCampaign(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	startTimeStr := r.FormValue("start_time")
	endTimeStr := r.FormValue("end_time")

	start, err := time.Parse("2006-01-02T15:04", startTimeStr)
	if err != nil {
		log.Printf("Error parsing start_time '%s': %v", startTimeStr, err)
		http.Error(w, "Invalid start_time format", http.StatusBadRequest)
		return
	}

	end, err := time.Parse("2006-01-02T15:04", endTimeStr)
	if err != nil {
		log.Printf("Error parsing end_time '%s': %v", endTimeStr, err)
		http.Error(w, "Invalid end_time format", http.StatusBadRequest)
		return
	}

	log.Println("start", start)
	log.Println("end", end)

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

// EditCampaign shows the edit form for a campaign
func (h *Handler) EditCampaign(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract campaign ID from path (e.g., /campaigns/123/edit)
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 3 || pathParts[0] != "campaigns" || pathParts[2] != "edit" {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}
	campaignID := pathParts[1]

	campaign, err := h.service.GetCampaign(campaignID)
	if err != nil {
		http.Error(w, "Campaign not found", http.StatusNotFound)
		return
	}

	availableAds, err := h.service.GetAvailableAds()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get current ad's media URL if campaign has ads
	currentMediaURL := ""
	if len(campaign.Ads) > 0 {
		currentMediaURL = campaign.Ads[0].MediaURL
	}

	// Get all campaigns for the table
	allCampaigns, err := h.service.ListCampaigns()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := struct {
		Campaign        *models.Campaign
		Campaigns       []models.Campaign
		AvailableAds    []models.Ad
		CurrentMediaURL string
	}{
		Campaign:        campaign,
		Campaigns:       allCampaigns,
		AvailableAds:    availableAds,
		CurrentMediaURL: currentMediaURL,
	}

	tmpl := template.Must(template.ParseFiles("web/templates/layout.html", "web/templates/campaigns.html"))
	tmpl.Execute(w, data)
}

// UpdateCampaign handles campaign update form submission
func (h *Handler) UpdateCampaign(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract campaign ID from path (e.g., /campaigns/123/update)
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 3 || pathParts[0] != "campaigns" || pathParts[2] != "update" {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}
	campaignID := pathParts[1]

	startTimeStr := r.FormValue("start_time")
	endTimeStr := r.FormValue("end_time")

	start, err := time.Parse("2006-01-02T15:04", startTimeStr)
	if err != nil {
		log.Printf("Error parsing start_time '%s': %v", startTimeStr, err)
		http.Error(w, "Invalid start_time format", http.StatusBadRequest)
		return
	}

	end, err := time.Parse("2006-01-02T15:04", endTimeStr)
	if err != nil {
		log.Printf("Error parsing end_time '%s': %v", endTimeStr, err)
		http.Error(w, "Invalid end_time format", http.StatusBadRequest)
		return
	}

	// Get the selected available ad by media_url
	mediaURL := r.FormValue("media_url")
	availableAd, err := h.service.GetAvailableAdByMediaURL(mediaURL)
	if err != nil {
		http.Error(w, "Invalid media URL selected", http.StatusBadRequest)
		return
	}

	// Create updated campaign
	ads := []models.Ad{
		{
			MediaURL:        availableAd.MediaURL,
			DurationSeconds: availableAd.DurationSeconds,
			CreativeID:      availableAd.CreativeID,
		},
	}
	campaign := models.Campaign{
		ID:        campaignID,
		Name:      r.FormValue("name"),
		StartTime: start,
		EndTime:   end,
		TargetDMA: r.FormValue("target_dma"),
		Ads:       ads,
	}

	if err := h.service.UpdateCampaign(campaign); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/campaigns", http.StatusSeeOther)
}

// UpdateCampaignAPI handles REST API campaign updates (PUT/PATCH)
func (h *Handler) UpdateCampaignAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != "PUT" && r.Method != "PATCH" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract campaign ID from path (e.g., /api/campaigns/123)
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 3 || pathParts[0] != "api" || pathParts[1] != "campaigns" {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}
	campaignID := pathParts[2]

	// Parse JSON request body
	var campaign models.Campaign
	if err := json.NewDecoder(r.Body).Decode(&campaign); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Ensure the ID matches
	campaign.ID = campaignID

	// Validate required fields
	if campaign.Name == "" {
		http.Error(w, "Campaign name is required", http.StatusBadRequest)
		return
	}
	if campaign.StartTime.IsZero() || campaign.EndTime.IsZero() {
		http.Error(w, "Start time and end time are required", http.StatusBadRequest)
		return
	}
	if campaign.TargetDMA == "" {
		http.Error(w, "Target DMA is required", http.StatusBadRequest)
		return
	}

	// If ads are provided, validate them; otherwise keep existing ads
	if len(campaign.Ads) == 0 {
		// Get existing campaign to preserve ads
		existing, err := h.service.GetCampaign(campaignID)
		if err != nil {
			http.Error(w, "Campaign not found", http.StatusNotFound)
			return
		}
		campaign.Ads = existing.Ads
	}

	if err := h.service.UpdateCampaign(campaign); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Return updated campaign
	updated, err := h.service.GetCampaign(campaignID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updated)
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

// ListRequestLogs renders the request logs UI page
func (h *Handler) ListRequestLogs(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters for filters
	limit := 50 // default limit for UI
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsedLimit, err := strconv.ParseInt(limitStr, 10, 32); err == nil && parsedLimit > 0 && parsedLimit <= 200 {
			limit = int(parsedLimit)
		}
	}

	offset := 0
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if parsedOffset, err := strconv.ParseInt(offsetStr, 10, 32); err == nil {
			offset = int(parsedOffset)
		}
	}

	methodFilter := r.URL.Query().Get("method")
	pathFilter := r.URL.Query().Get("path")

	var startTime *time.Time
	if startTimeStr := r.URL.Query().Get("start_time"); startTimeStr != "" {
		if parsed, err := time.Parse("2006-01-02T15:04", startTimeStr); err == nil {
			startTime = &parsed
		}
	}

	var endTime *time.Time
	if endTimeStr := r.URL.Query().Get("end_time"); endTimeStr != "" {
		if parsed, err := time.Parse("2006-01-02T15:04", endTimeStr); err == nil {
			endTime = &parsed
		}
	}

	// Get logs
	logs, err := h.store.GetRequestLogs(limit, offset, methodFilter, pathFilter, startTime, endTime)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get total count
	totalCount, err := h.store.GetRequestLogCount(methodFilter, pathFilter, startTime, endTime)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Calculate pagination values
	nextOffset := offset + limit
	prevOffset := offset - limit
	if prevOffset < 0 {
		prevOffset = 0
	}
	showingEnd := offset + len(logs)
	if showingEnd > totalCount {
		showingEnd = totalCount
	}

	// Convert logs to JSON for JavaScript
	logsJSON, _ := json.Marshal(logs)

	data := struct {
		Logs       []models.RequestLog
		LogsJSON   string
		Total      int
		Limit      int
		Offset     int
		NextOffset int
		PrevOffset int
		ShowingEnd int
		HasMore    bool
		Method     string
		Path       string
		StartTime  string
		EndTime    string
	}{
		Logs:       logs,
		LogsJSON:   string(logsJSON),
		Total:      totalCount,
		Limit:      limit,
		Offset:     offset,
		NextOffset: nextOffset,
		PrevOffset: prevOffset,
		ShowingEnd: showingEnd,
		HasMore:    offset+len(logs) < totalCount,
		Method:     methodFilter,
		Path:       pathFilter,
		StartTime:  r.URL.Query().Get("start_time"),
		EndTime:    r.URL.Query().Get("end_time"),
	}

	// Create template with custom functions
	funcMap := template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
	}
	tmpl := template.Must(template.New("").Funcs(funcMap).ParseFiles("web/templates/layout.html", "web/templates/logs.html"))
	tmpl.Execute(w, data)
}

// QueryRequestLogs returns request logs with optional filters (JSON API)
func (h *Handler) QueryRequestLogs(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	limit := 100 // default limit
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsedLimit, err := strconv.ParseInt(limitStr, 10, 32); err == nil && parsedLimit > 0 && parsedLimit <= 1000 {
			limit = int(parsedLimit)
		}
	}

	offset := 0
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if parsedOffset, err := strconv.ParseInt(offsetStr, 10, 32); err == nil {
			offset = int(parsedOffset)
		}
	}

	methodFilter := r.URL.Query().Get("method")
	pathFilter := r.URL.Query().Get("path")

	var startTime *time.Time
	if startTimeStr := r.URL.Query().Get("start_time"); startTimeStr != "" {
		if parsed, err := time.Parse(time.RFC3339, startTimeStr); err == nil {
			startTime = &parsed
		}
	}

	var endTime *time.Time
	if endTimeStr := r.URL.Query().Get("end_time"); endTimeStr != "" {
		if parsed, err := time.Parse(time.RFC3339, endTimeStr); err == nil {
			endTime = &parsed
		}
	}

	// Get logs
	logs, err := h.store.GetRequestLogs(limit, offset, methodFilter, pathFilter, startTime, endTime)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get total count
	totalCount, err := h.store.GetRequestLogCount(methodFilter, pathFilter, startTime, endTime)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Return JSON response
	response := map[string]interface{}{
		"logs":     logs,
		"total":    totalCount,
		"limit":    limit,
		"offset":   offset,
		"has_more": offset+len(logs) < totalCount,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
