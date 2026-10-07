CREATE EXTENSION IF NOT EXISTS postgis;

CREATE TABLE IF NOT EXISTS properties (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    price BIGINT NOT NULL CHECK (price >= 0),
    price_per_ping DOUBLE PRECISION NOT NULL CHECK (price_per_ping >= 0),
    area DOUBLE PRECISION NOT NULL CHECK (area > 0),
    rooms INTEGER NOT NULL CHECK (rooms >= 0),
    location GEOGRAPHY(POINT, 4326) NOT NULL,
    address TEXT NOT NULL,
    property_type TEXT NOT NULL,
    listed_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS properties_location_gist_idx ON properties USING GIST (location);
CREATE INDEX IF NOT EXISTS properties_price_idx ON properties (price);
CREATE INDEX IF NOT EXISTS properties_area_idx ON properties (area);
CREATE INDEX IF NOT EXISTS properties_type_idx ON properties (property_type);
CREATE INDEX IF NOT EXISTS properties_listed_at_idx ON properties (listed_at DESC);
