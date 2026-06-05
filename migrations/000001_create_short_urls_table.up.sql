CREATE TABLE short_urls (
    id SERIAL PRIMARY KEY,
    original_url VARCHAR(255) NOT NULL,
    short_url_id VARCHAR(255) NOT NULL
);

CREATE UNIQUE INDEX idx_url ON short_urls(short_url_id);