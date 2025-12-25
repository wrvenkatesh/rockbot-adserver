CREATE TABLE IF NOT EXISTS campaigns (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    start_time DATETIME NOT NULL,
    end_time DATETIME NOT NULL,
    target_dma TEXT NOT NULL -- '*' for all, or specific code like '10'
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