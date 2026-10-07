-- District areas approximated from door-number points (see api/internal/districtmap),
-- recomputed after every door-number import.
CREATE TABLE IF NOT EXISTS district_boundaries (
    city_code TEXT NOT NULL,
    district TEXT NOT NULL,
    cells INTEGER NOT NULL,
    coordinates JSONB NOT NULL, -- GeoJSON MultiPolygon coordinates
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (city_code, district)
);
