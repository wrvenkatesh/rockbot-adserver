package service

import (
	"encoding/xml"
	"fmt"
	"rockbot-adserver/internal/models"
	"rockbot-adserver/internal/store"
	"time"

	"github.com/google/uuid"
)

type AdService struct {
	store *store.Store
}

func NewAdService(store *store.Store) *AdService {
	return &AdService{store: store}
}

func (s *AdService) CreateCampaign(c models.Campaign) error {
	if c.ID == "" {
		c.ID = uuid.New().String()
	}
	// Assign IDs to ads if missing
	for i := range c.Ads {
		if c.Ads[i].ID == "" {
			c.Ads[i].ID = uuid.New().String()
		}
	}
	return s.store.CreateCampaign(c)
}

func (s *AdService) GetAdsForClient(clientID, dma string) (string, error) {
	now := time.Now()
	// 1. Get Active Campaigns for DMA
	campaigns, err := s.store.GetActiveCampaigns(dma, now)
	if err != nil {
		return "", err
	}

	// 2. Rate Limiting Check
	// "Each unique client must be served no more than 5 minutes (300 seconds) of total ad duration within the current hour."
	oneHourAgo := now.Add(-1 * time.Hour)
	currentDuration, err := s.store.GetClientImpressionsDuration(clientID, oneHourAgo)
	if err != nil {
		return "", err
	}

	remainingDuration := 300 - currentDuration
	if remainingDuration <= 0 {
		return s.GenerateVAST(nil), nil // Return empty VAST
	}

	// 3. Select Ads
	var selectedAds []models.Ad
	for _, c := range campaigns {
		for _, ad := range c.Ads {
			if ad.DurationSeconds <= remainingDuration {
				selectedAds = append(selectedAds, ad)
				remainingDuration -= ad.DurationSeconds

				// Record Impression immediately (simplified logic, usually done on ping)
				// Note: User requirements say "Ads served must belong...", implementation detail: we count them as served when we return them for simplicity here,
				// or we should rely on client pings. For a backend logic test, pre-recording or separate endpoint is common.
				// Given the "Rate limiting" constraint is strictly about "served", better to count it now.
				s.store.RecordImpression(models.Impression{
					ID:              uuid.New().String(),
					ClientID:        clientID,
					AdID:            ad.ID,
					DurationSeconds: ad.DurationSeconds,
					Timestamp:       time.Now(),
				})
			}
			if remainingDuration <= 0 {
				break
			}
		}
		if remainingDuration <= 0 {
			break
		}
	}

	return s.GenerateVAST(selectedAds), nil
}

func (s *AdService) GenerateVAST(ads []models.Ad) string {
	vast := models.VAST{
		Version: "3.0",
		Ad:      make([]models.VASTAd, len(ads)),
	}

	for i, ad := range ads {
		vast.Ad[i] = models.VASTAd{
			ID: ad.ID,
			InLine: &models.InLine{
				AdSystem: "Rockbot Ad Server",
				AdTitle:  "Inline Video Ad",
				Creatives: models.Creatives{
					Creative: []models.Creative{
						{
							ID: ad.CreativeID,
							Linear: &models.Linear{
								Duration: fmt.Sprintf("00:00:%02d", ad.DurationSeconds),
								MediaFiles: models.MediaFiles{
									MediaFile: []models.MediaFile{
										{
											Delivery: "progressive",
											Type:     "video/mp4",
											Width:    1280,
											Height:   720,
											URL:      ad.MediaURL,
										},
									},
								},
							},
						},
					},
				},
			},
		}
	}

	output, _ := xml.MarshalIndent(vast, "", "  ")
	return xml.Header + string(output)
}

func (s *AdService) ListCampaigns() ([]models.Campaign, error) {
	return s.store.GetAllCampaigns()
}

func (s *AdService) GetAvailableAds() ([]models.Ad, error) {
	return s.store.GetAvailableAds()
}

func (s *AdService) GetAvailableAdByMediaURL(mediaURL string) (*models.Ad, error) {
	return s.store.GetAvailableAdByMediaURL(mediaURL)
}

func (s *AdService) GetCampaign(id string) (*models.Campaign, error) {
	return s.store.GetCampaignByID(id)
}

func (s *AdService) UpdateCampaign(c models.Campaign) error {
	// Ensure campaign has an ID
	if c.ID == "" {
		return fmt.Errorf("campaign ID is required")
	}
	// Assign IDs to ads if missing
	for i := range c.Ads {
		if c.Ads[i].ID == "" {
			c.Ads[i].ID = uuid.New().String()
		}
	}
	return s.store.UpdateCampaign(c)
}
