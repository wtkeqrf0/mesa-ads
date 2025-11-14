CREATE TABLE IF NOT EXISTS campaigns (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    start_date TIMESTAMP NOT NULL,
    end_date TIMESTAMP NOT NULL,
    daily_budget BIGINT NOT NULL,
    total_budget BIGINT NOT NULL,
    remaining_daily_budget BIGINT NOT NULL,
    remaining_total_budget BIGINT NOT NULL,
    cpm_bid BIGINT NOT NULL,
    cpc_bid BIGINT NOT NULL,
    status VARCHAR(20) NOT NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS creatives (
    id SERIAL PRIMARY KEY,
    campaign_id INT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    video_url TEXT NOT NULL,
    landing_url TEXT NOT NULL,
    duration INT NOT NULL,
    language VARCHAR(10),
    category TEXT,
    placement TEXT,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS campaign_targeting (
    campaign_id INT PRIMARY KEY REFERENCES campaigns(id) ON DELETE CASCADE,
    data JSONB NOT NULL
);

CREATE TABLE IF NOT EXISTS impressions (
    id SERIAL PRIMARY KEY,
    token TEXT UNIQUE NOT NULL,
    creative_id INT NOT NULL REFERENCES creatives(id) ON DELETE CASCADE,
    campaign_id INT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL,
    cost BIGINT NOT NULL,
    created_at TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS clicks (
    id SERIAL PRIMARY KEY,
    token TEXT UNIQUE NOT NULL,
    impression_id INT REFERENCES impressions(id) ON DELETE SET NULL,
    creative_id INT NOT NULL REFERENCES creatives(id) ON DELETE CASCADE,
    campaign_id INT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL,
    cost BIGINT NOT NULL,
    created_at TIMESTAMP NOT NULL
);