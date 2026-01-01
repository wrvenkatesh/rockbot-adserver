package models

import "time"

type Campaign struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	TargetDMA string    `json:"target_dma"` // "10" or "*"
	Ads       []Ad      `json:"ads,omitempty"`
}

type Ad struct {
	ID              string `json:"id"`
	CampaignID      string `json:"campaign_id"`
	MediaURL        string `json:"media_url"`
	DurationSeconds int    `json:"duration_seconds"`
	CreativeID      string `json:"creative_id"`
}

type Impression struct {
	ID              string    `json:"id"`
	ClientID        string    `json:"client_id"`
	AdID            string    `json:"ad_id"`
	DurationSeconds int       `json:"duration_seconds"`
	Timestamp       time.Time `json:"timestamp"`
}

// VAST Structures for response generation
type VAST struct {
	Version string `xml:"version,attr"`
	Ad      []VASTAd `xml:"Ad"`
}

type VASTAd struct {
	ID     string  `xml:"id,attr"`
	InLine *InLine `xml:"InLine,omitempty"`
}

type InLine struct {
	AdSystem    string      `xml:"AdSystem"`
	AdTitle     string      `xml:"AdTitle"`
	Creatives   Creatives   `xml:"Creatives"`
	Impression  string      `xml:"Impression"` // URL to ping
}

type Creatives struct {
	Creative []Creative `xml:"Creative"`
}

type Creative struct {
	ID     string  `xml:"id,attr"`
	Linear *Linear `xml:"Linear,omitempty"`
}

type Linear struct {
	Duration   string     `xml:"Duration"` // HH:MM:SS
	MediaFiles MediaFiles `xml:"MediaFiles"`
}

type MediaFiles struct {
	MediaFile []MediaFile `xml:"MediaFile"`
}

type MediaFile struct {
	Delivery string `xml:"delivery,attr"`
	Type     string `xml:"type,attr"`
	Width    int    `xml:"width,attr"`
	Height   int    `xml:"height,attr"`
	URL      string `xml:",chardata"`
}

type RequestLog struct {
	ID              string    `json:"id"`
	Method          string    `json:"method"`
	Path            string    `json:"path"`
	QueryParams     string    `json:"query_params"`
	RequestHeaders  string    `json:"request_headers"`
	RequestBody     string    `json:"request_body"`
	ResponseStatus  int       `json:"response_status"`
	ResponseHeaders string    `json:"response_headers"`
	ResponseBody    string    `json:"response_body"`
	DurationMs      int64     `json:"duration_ms"`
	Timestamp       time.Time `json:"timestamp"`
	RemoteAddr      string    `json:"remote_addr"`
	UserAgent       string    `json:"user_agent"`
}
