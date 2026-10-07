CREATE EXTENSION IF NOT EXISTS postgis;

CREATE TABLE IF NOT EXISTS import_runs (
    id BIGSERIAL PRIMARY KEY,
    source TEXT NOT NULL,
    dataset TEXT NOT NULL,
    season TEXT NOT NULL,
    city_code TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('running', 'succeeded', 'failed')),
    rows_read INTEGER NOT NULL DEFAULT 0,
    rows_skipped INTEGER NOT NULL DEFAULT 0,
    rows_upserted INTEGER NOT NULL DEFAULT 0,
    rows_geocode_failed INTEGER NOT NULL DEFAULT 0,
    detail JSONB NOT NULL DEFAULT '{}', -- skip reasons and geocode precision counts
    error TEXT,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS import_runs_season_city_idx ON import_runs (season, city_code, started_at DESC);

-- Real-estate sale transactions from the MOI actual price registration open data (實價登錄).
CREATE TABLE IF NOT EXISTS transactions (
    id TEXT PRIMARY KEY, -- 編號, stable across official corrections
    city_code TEXT NOT NULL,
    district TEXT NOT NULL,
    address TEXT NOT NULL,
    transaction_type TEXT NOT NULL,
    building_type TEXT NOT NULL,
    main_use TEXT NOT NULL,
    total_price BIGINT NOT NULL CHECK (total_price > 0),
    parking_price BIGINT NOT NULL CHECK (parking_price >= 0),
    building_price BIGINT NOT NULL CHECK (building_price > 0),
    building_area_ping DOUBLE PRECISION NOT NULL CHECK (building_area_ping > 0),
    price_per_ping DOUBLE PRECISION NOT NULL CHECK (price_per_ping > 0),
    rooms INTEGER NOT NULL CHECK (rooms >= 0),
    halls INTEGER NOT NULL CHECK (halls >= 0),
    baths INTEGER NOT NULL CHECK (baths >= 0),
    floor_label TEXT NOT NULL,
    total_floors INTEGER CHECK (total_floors > 0),
    completed_on DATE,
    transacted_on DATE NOT NULL,
    remarks TEXT NOT NULL DEFAULT '',
    special_reasons TEXT[] NOT NULL DEFAULT '{}',
    is_special BOOLEAN GENERATED ALWAYS AS (cardinality(special_reasons) > 0) STORED,
    location GEOGRAPHY(POINT, 4326),
    geocode_precision TEXT CHECK (geocode_precision IN ('address', 'nearby', 'road', 'district')),
    season TEXT NOT NULL,
    import_run_id BIGINT REFERENCES import_runs (id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS transactions_location_gist_idx ON transactions USING GIST (location);
CREATE INDEX IF NOT EXISTS transactions_transacted_on_idx ON transactions (transacted_on DESC);
CREATE INDEX IF NOT EXISTS transactions_district_idx ON transactions (city_code, district);
CREATE INDEX IF NOT EXISTS transactions_price_per_ping_idx ON transactions (price_per_ping);
CREATE INDEX IF NOT EXISTS transactions_building_type_idx ON transactions (building_type);
