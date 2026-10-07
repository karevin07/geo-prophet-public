-- Presales (預售屋) from the MOI actual price registration open data, imported from the
-- same archives as sales. transacted_on is the contract date. Before season 110S3 (July
-- 2021) presales were registered in bulk after the sales agency's contract ended.
CREATE TABLE IF NOT EXISTS presales (
    id TEXT PRIMARY KEY, -- 編號
    city_code TEXT NOT NULL,
    district TEXT NOT NULL,
    address TEXT NOT NULL, -- often only a road or lane
    project_name TEXT NOT NULL DEFAULT '', -- 建案名稱, from about season 108S1
    project_key TEXT, -- city|district|name; NULL without a name
    unit_label TEXT NOT NULL DEFAULT '', -- 棟及號
    cancellation TEXT NOT NULL DEFAULT '', -- 解約情形, e.g. 1150329全部解約
    cancelled_on DATE,
    is_cancelled BOOLEAN GENERATED ALWAYS AS (cancellation <> '') STORED,
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
    completed_on DATE, -- empty for presales; kept so rows share the sales layout
    transacted_on DATE NOT NULL,
    remarks TEXT NOT NULL DEFAULT '',
    special_reasons TEXT[] NOT NULL DEFAULT '{}',
    is_special BOOLEAN GENERATED ALWAYS AS (cardinality(special_reasons) > 0) STORED,
    -- Same categories as transactions (000005).
    building_category TEXT GENERATED ALWAYS AS (
        CASE split_part(building_type, '(', 1)
            WHEN '住宅大樓' THEN '住宅大樓'
            WHEN '華廈' THEN '華廈'
            WHEN '公寓' THEN '公寓'
            WHEN '透天厝' THEN '透天厝'
            ELSE '其他'
        END
    ) STORED,
    location GEOGRAPHY(POINT, 4326),
    geocode_precision TEXT CHECK (geocode_precision IN ('address', 'nearby', 'road', 'district')),
    season TEXT NOT NULL,
    import_run_id BIGINT REFERENCES import_runs (id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS presales_project_idx ON presales (project_key, transacted_on);
CREATE INDEX IF NOT EXISTS presales_district_idx ON presales (city_code, district, transacted_on);

-- One row per named project, located by its most precise presale (then the latest).
CREATE TABLE IF NOT EXISTS presale_projects (
    project_key TEXT PRIMARY KEY,
    city_code TEXT NOT NULL,
    district TEXT NOT NULL,
    name TEXT NOT NULL,
    address TEXT NOT NULL,
    location GEOGRAPHY(POINT, 4326),
    geocode_precision TEXT
);

CREATE INDEX IF NOT EXISTS presale_projects_city_idx ON presale_projects (city_code, district);
CREATE INDEX IF NOT EXISTS presale_projects_location_idx ON presale_projects USING GIST (location);

-- Rebuilds the given projects from their presales; the importer calls it after each
-- upsert with the projects it touched.
CREATE OR REPLACE FUNCTION refresh_presale_projects(keys TEXT[]) RETURNS VOID LANGUAGE SQL AS $$
    DELETE FROM presale_projects WHERE project_key = ANY (keys);
    INSERT INTO presale_projects (project_key, city_code, district, name, address, location, geocode_precision)
    SELECT DISTINCT ON (project_key) project_key, city_code, district, project_name, address, location, geocode_precision
    FROM presales
    WHERE project_key = ANY (keys)
    ORDER BY project_key,
        CASE geocode_precision WHEN 'address' THEN 0 WHEN 'nearby' THEN 1 WHEN 'road' THEN 2 WHEN 'district' THEN 3 ELSE 4 END,
        transacted_on DESC, id;
$$;
