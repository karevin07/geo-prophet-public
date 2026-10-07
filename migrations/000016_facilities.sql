-- Everyday facilities and nuisances near homes: shops, markets, parks, libraries and
-- nuisances from OpenStreetMap; schools from the education ministry's lists; hospitals
-- and clinics from the health insurance lists. Each source is replaced as a whole.
CREATE TABLE IF NOT EXISTS facilities (
    id TEXT PRIMARY KEY, -- e.g. osm:way/123, nhi:0101090517, moe:elementary:011601
    source TEXT NOT NULL CHECK (source IN ('osm', 'nhi', 'moe')),
    category TEXT NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    brand TEXT NOT NULL DEFAULT '',
    city_code TEXT NOT NULL,
    district TEXT NOT NULL,
    address TEXT NOT NULL DEFAULT '',
    location GEOGRAPHY(POINT, 4326) NOT NULL,
    -- The outline of parks, cemeteries and other areas; distances are measured to it.
    area GEOGRAPHY(POLYGON, 4326),
    area_m2 DOUBLE PRECISION,
    geocode_precision TEXT, -- for facilities located from an address
    import_run_id BIGINT REFERENCES import_runs (id),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS facilities_location_idx ON facilities USING GIST (location);
CREATE INDEX IF NOT EXISTS facilities_area_idx ON facilities USING GIST (area) WHERE area IS NOT NULL;
CREATE INDEX IF NOT EXISTS facilities_source_idx ON facilities (source);
