package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"rockbot-adserver/internal/models"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Store struct {
	db *sql.DB
}

func NewStore(dbPath string) (*Store, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create db directory: %w", err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	s := &Store{db: db}
	if err := s.initSchema(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Store) initSchema() error {
	// Simple schema init - in production we'd use migrations
	schema := `
	CREATE TABLE IF NOT EXISTS campaigns (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		start_time DATETIME NOT NULL,
		end_time DATETIME NOT NULL,
		target_dma TEXT NOT NULL
	);
	CREATE TABLE IF NOT EXISTS ads (
		id TEXT PRIMARY KEY,
		campaign_id TEXT,
		media_url TEXT NOT NULL,
		duration_seconds INTEGER NOT NULL,
		creative_id TEXT NOT NULL,
		FOREIGN KEY(campaign_id) REFERENCES campaigns(id) ON DELETE CASCADE
	);
	CREATE TABLE IF NOT EXISTS impressions (
		id TEXT PRIMARY KEY,
		client_id TEXT NOT NULL,
		ad_id TEXT NOT NULL,
		duration_seconds INTEGER NOT NULL,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_impressions_client_ts ON impressions(client_id, timestamp);
	CREATE TABLE IF NOT EXISTS request_logs (
		id TEXT PRIMARY KEY,
		method TEXT NOT NULL,
		path TEXT NOT NULL,
		query_params TEXT,
		request_headers TEXT,
		request_body TEXT,
		response_status INTEGER NOT NULL,
		response_headers TEXT,
		response_body TEXT,
		duration_ms INTEGER NOT NULL,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		remote_addr TEXT,
		user_agent TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_request_logs_timestamp ON request_logs(timestamp);
	CREATE INDEX IF NOT EXISTS idx_request_logs_path ON request_logs(path);
	CREATE INDEX IF NOT EXISTS idx_request_logs_method ON request_logs(method);
	`
	_, err := s.db.Exec(schema)
	return err
}

func (s *Store) CreateCampaign(c models.Campaign) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec("INSERT INTO campaigns (id, name, start_time, end_time, target_dma) VALUES (?, ?, ?, ?, ?)",
		c.ID, c.Name, c.StartTime, c.EndTime, c.TargetDMA)
	if err != nil {
		return err
	}

	for _, ad := range c.Ads {
		_, err = tx.Exec("INSERT INTO ads (id, campaign_id, media_url, duration_seconds, creative_id) VALUES (?, ?, ?, ?, ?)",
			ad.ID, c.ID, ad.MediaURL, ad.DurationSeconds, ad.CreativeID)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Store) GetActiveCampaigns(dma string, now time.Time) ([]models.Campaign, error) {
	query := `
		SELECT c.id, c.name, c.start_time, c.end_time, c.target_dma,
		       a.id, a.media_url, a.duration_seconds, a.creative_id
		FROM campaigns c
		JOIN ads a ON c.id = a.campaign_id
		WHERE ? BETWEEN c.start_time AND c.end_time
		AND (c.target_dma = '*' OR c.target_dma = ?)
	`
	rows, err := s.db.Query(query, now, dma)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	campaignMap := make(map[string]*models.Campaign)

	for rows.Next() {
		var cID, cName, cDMA string
		var cStart, cEnd time.Time
		var aID, aUrl, aCreative string
		var aDur int

		err := rows.Scan(&cID, &cName, &cStart, &cEnd, &cDMA, &aID, &aUrl, &aDur, &aCreative)
		if err != nil {
			return nil, err
		}

		if _, exists := campaignMap[cID]; !exists {
			campaignMap[cID] = &models.Campaign{
				ID:        cID,
				Name:      cName,
				StartTime: cStart,
				EndTime:   cEnd,
				TargetDMA: cDMA,
				Ads:       []models.Ad{},
			}
		}

		campaignMap[cID].Ads = append(campaignMap[cID].Ads, models.Ad{
			ID:              aID,
			CampaignID:      cID,
			MediaURL:        aUrl,
			DurationSeconds: aDur,
			CreativeID:      aCreative,
		})
	}

	var result []models.Campaign
	for _, c := range campaignMap {
		result = append(result, *c)
	}
	return result, nil
}

func (s *Store) GetClientImpressionsDuration(clientID string, since time.Time) (int, error) {
	var total int
	err := s.db.QueryRow("SELECT COALESCE(SUM(duration_seconds), 0) FROM impressions WHERE client_id = ? AND timestamp > ?", clientID, since).Scan(&total)
	return total, err
}

func (s *Store) RecordImpression(imp models.Impression) error {
	_, err := s.db.Exec("INSERT INTO impressions (id, client_id, ad_id, duration_seconds, timestamp) VALUES (?, ?, ?, ?, ?)",
		imp.ID, imp.ClientID, imp.AdID, imp.DurationSeconds, imp.Timestamp)
	return err
}

// GetAllCampaigns for UI
func (s *Store) GetAllCampaigns() ([]models.Campaign, error) {
	rows, err := s.db.Query("SELECT id, name, start_time, end_time, target_dma FROM campaigns ORDER BY start_time DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var campaigns []models.Campaign
	for rows.Next() {
		var c models.Campaign
		if err := rows.Scan(&c.ID, &c.Name, &c.StartTime, &c.EndTime, &c.TargetDMA); err != nil {
			return nil, err
		}
		campaigns = append(campaigns, c)
	}
	return campaigns, nil
}

// SeedAvailableAds seeds available ads (with NULL campaign_id) if they don't exist
func (s *Store) SeedAvailableAds(ads []models.Ad) error {
	for _, ad := range ads {
		// Check if ad with this media_url already exists
		var exists bool
		err := s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM ads WHERE media_url = ? AND campaign_id IS NULL)", ad.MediaURL).Scan(&exists)
		if err != nil {
			return err
		}
		
		if !exists {
			_, err = s.db.Exec("INSERT INTO ads (id, campaign_id, media_url, duration_seconds, creative_id) VALUES (?, NULL, ?, ?, ?)",
				ad.ID, ad.MediaURL, ad.DurationSeconds, ad.CreativeID)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// GetAvailableAds returns all ads that are not linked to any campaign (available for selection)
func (s *Store) GetAvailableAds() ([]models.Ad, error) {
	rows, err := s.db.Query("SELECT id, campaign_id, media_url, duration_seconds, creative_id FROM ads WHERE campaign_id IS NULL ORDER BY media_url")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ads []models.Ad
	for rows.Next() {
		var ad models.Ad
		var campaignID sql.NullString
		if err := rows.Scan(&ad.ID, &campaignID, &ad.MediaURL, &ad.DurationSeconds, &ad.CreativeID); err != nil {
			return nil, err
		}
		if campaignID.Valid {
			ad.CampaignID = campaignID.String
		}
		ads = append(ads, ad)
	}
	return ads, nil
}

// GetAvailableAdByMediaURL returns an available ad (with NULL campaign_id) by media_url
func (s *Store) GetAvailableAdByMediaURL(mediaURL string) (*models.Ad, error) {
	var ad models.Ad
	var campaignID sql.NullString
	err := s.db.QueryRow("SELECT id, campaign_id, media_url, duration_seconds, creative_id FROM ads WHERE media_url = ? AND campaign_id IS NULL", mediaURL).
		Scan(&ad.ID, &campaignID, &ad.MediaURL, &ad.DurationSeconds, &ad.CreativeID)
	if err != nil {
		return nil, err
	}
	if campaignID.Valid {
		ad.CampaignID = campaignID.String
	}
	return &ad, nil
}

// SaveRequestLog saves a request/response log to the database
func (s *Store) SaveRequestLog(log models.RequestLog) error {
	_, err := s.db.Exec(`
		INSERT INTO request_logs (
			id, method, path, query_params, request_headers, request_body,
			response_status, response_headers, response_body, duration_ms,
			timestamp, remote_addr, user_agent
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		log.ID, log.Method, log.Path, log.QueryParams, log.RequestHeaders, log.RequestBody,
		log.ResponseStatus, log.ResponseHeaders, log.ResponseBody, log.DurationMs,
		log.Timestamp, log.RemoteAddr, log.UserAgent)
	return err
}

// GetRequestLogs retrieves request logs with optional filters
func (s *Store) GetRequestLogs(limit int, offset int, methodFilter string, pathFilter string, startTime *time.Time, endTime *time.Time) ([]models.RequestLog, error) {
	query := "SELECT id, method, path, query_params, request_headers, request_body, response_status, response_headers, response_body, duration_ms, timestamp, remote_addr, user_agent FROM request_logs WHERE 1=1"
	args := []interface{}{}

	if methodFilter != "" {
		query += " AND method = ?"
		args = append(args, methodFilter)
	}
	if pathFilter != "" {
		query += " AND path LIKE ?"
		args = append(args, "%"+pathFilter+"%")
	}
	if startTime != nil {
		query += " AND timestamp >= ?"
		args = append(args, *startTime)
	}
	if endTime != nil {
		query += " AND timestamp <= ?"
		args = append(args, *endTime)
	}

	query += " ORDER BY timestamp DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []models.RequestLog
	for rows.Next() {
		var log models.RequestLog
		err := rows.Scan(
			&log.ID, &log.Method, &log.Path, &log.QueryParams, &log.RequestHeaders, &log.RequestBody,
			&log.ResponseStatus, &log.ResponseHeaders, &log.ResponseBody, &log.DurationMs,
			&log.Timestamp, &log.RemoteAddr, &log.UserAgent)
		if err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}
	return logs, nil
}

// GetRequestLogCount returns the total count of request logs matching filters
func (s *Store) GetRequestLogCount(methodFilter string, pathFilter string, startTime *time.Time, endTime *time.Time) (int, error) {
	query := "SELECT COUNT(*) FROM request_logs WHERE 1=1"
	args := []interface{}{}

	if methodFilter != "" {
		query += " AND method = ?"
		args = append(args, methodFilter)
	}
	if pathFilter != "" {
		query += " AND path LIKE ?"
		args = append(args, "%"+pathFilter+"%")
	}
	if startTime != nil {
		query += " AND timestamp >= ?"
		args = append(args, *startTime)
	}
	if endTime != nil {
		query += " AND timestamp <= ?"
		args = append(args, *endTime)
	}

	var count int
	err := s.db.QueryRow(query, args...).Scan(&count)
	return count, err
}
