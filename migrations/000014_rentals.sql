-- Registered leases from the MOI actual price registration open data (實價登錄 租賃),
-- imported from the same archives as sales. Only leases handled by a rental management
-- business or an agent must be registered, so landlords renting directly are missing.
CREATE TABLE IF NOT EXISTS rentals (
    id TEXT PRIMARY KEY, -- 編號, stable across official corrections
    city_code TEXT NOT NULL,
    district TEXT NOT NULL,
    address TEXT NOT NULL,
    transaction_type TEXT NOT NULL, -- 交易標的
    rental_type TEXT, -- 出租型態; NULL before season 112S4
    -- A whole home rather than a room or part of one; inferred from the area and remarks
    -- when rental_type is NULL. Only whole homes have comparable rent per ping.
    whole_unit BOOLEAN NOT NULL,
    whole_unit_inferred BOOLEAN NOT NULL,
    social_housing BOOLEAN NOT NULL, -- social housing schemes cap rents
    rental_service TEXT NOT NULL DEFAULT '', -- 租賃住宅服務
    building_type TEXT NOT NULL,
    main_use TEXT NOT NULL,
    monthly_rent BIGINT NOT NULL CHECK (monthly_rent > 0),
    parking_rent BIGINT NOT NULL CHECK (parking_rent >= 0),
    building_rent BIGINT NOT NULL CHECK (building_rent > 0),
    building_area_ping DOUBLE PRECISION NOT NULL CHECK (building_area_ping > 0),
    rent_per_ping DOUBLE PRECISION NOT NULL CHECK (rent_per_ping > 0),
    rooms INTEGER NOT NULL CHECK (rooms >= 0),
    halls INTEGER NOT NULL CHECK (halls >= 0),
    baths INTEGER NOT NULL CHECK (baths >= 0),
    floor_label TEXT NOT NULL,
    total_floors INTEGER CHECK (total_floors > 0),
    completed_on DATE,
    leased_on DATE NOT NULL, -- 租賃年月日
    lease_start DATE,
    lease_end DATE,
    furnished BOOLEAN NOT NULL,
    has_manager BOOLEAN,
    has_elevator BOOLEAN,
    equipment TEXT NOT NULL DEFAULT '',
    renewal BOOLEAN NOT NULL,
    remarks TEXT NOT NULL DEFAULT '',
    special_reasons TEXT[] NOT NULL DEFAULT '{}',
    is_special BOOLEAN GENERATED ALWAYS AS (cardinality(special_reasons) > 0) STORED,
    -- Same categories and ages as transactions (000005).
    building_category TEXT GENERATED ALWAYS AS (
        CASE split_part(building_type, '(', 1)
            WHEN '住宅大樓' THEN '住宅大樓'
            WHEN '華廈' THEN '華廈'
            WHEN '公寓' THEN '公寓'
            WHEN '透天厝' THEN '透天厝'
            ELSE '其他'
        END
    ) STORED,
    building_age INTEGER GENERATED ALWAYS AS (
        CASE WHEN completed_on IS NULL OR completed_on > leased_on THEN NULL
             ELSE floor((leased_on - completed_on) / 365.25)::INTEGER
        END
    ) STORED,
    location GEOGRAPHY(POINT, 4326),
    geocode_precision TEXT CHECK (geocode_precision IN ('address', 'nearby', 'road', 'district')),
    season TEXT NOT NULL,
    import_run_id BIGINT REFERENCES import_runs (id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS rentals_location_gist_idx ON rentals USING GIST (location);
CREATE INDEX IF NOT EXISTS rentals_leased_on_idx ON rentals (leased_on DESC);
CREATE INDEX IF NOT EXISTS rentals_district_idx ON rentals (city_code, district, leased_on);

-- Straight-line metres from a lease to its stations' nearest exits, like
-- transaction_station_distances.
CREATE TABLE IF NOT EXISTS rental_station_distances (
    station_id TEXT NOT NULL REFERENCES transit_stations (id) ON DELETE CASCADE,
    rental_id TEXT NOT NULL,
    distance_m REAL NOT NULL,
    PRIMARY KEY (station_id, rental_id)
);

CREATE INDEX IF NOT EXISTS rental_station_distances_rental_idx ON rental_station_distances (rental_id);
