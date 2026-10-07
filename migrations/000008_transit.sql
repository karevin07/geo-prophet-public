-- Rail stations and lines from TDX (see api/internal/transit), replaced on each import.
CREATE TABLE IF NOT EXISTS transit_lines (
    id TEXT PRIMARY KEY, -- operator-LineID, e.g. TRTC-BL
    operator TEXT NOT NULL,
    system TEXT NOT NULL CHECK (system IN ('metro', 'lrt', 'tra', 'thsr')),
    name TEXT NOT NULL,
    name_en TEXT NOT NULL DEFAULT '',
    color TEXT NOT NULL,
    opened_on DATE, -- NULL when opened before the openings table starts (2015)
    geometry GEOGRAPHY(MULTILINESTRING, 4326) NOT NULL, -- clipped to the covered cities
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- A station merges platforms of one name close together, e.g. 板橋 for 台鐵, 高鐵 and MRT.
CREATE TABLE IF NOT EXISTS transit_stations (
    id TEXT PRIMARY KEY, -- ID of the first platform, e.g. TRTC-BL07
    name TEXT NOT NULL,
    name_en TEXT NOT NULL DEFAULT '',
    city_code TEXT NOT NULL,
    district TEXT NOT NULL,
    location GEOGRAPHY(POINT, 4326) NOT NULL,
    systems TEXT[] NOT NULL,
    opened_on DATE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- One row per platform and line it serves.
CREATE TABLE IF NOT EXISTS transit_station_lines (
    station_id TEXT NOT NULL REFERENCES transit_stations (id) ON DELETE CASCADE,
    platform_id TEXT NOT NULL, -- e.g. NTMC-Y16
    line_id TEXT NOT NULL REFERENCES transit_lines (id) ON DELETE CASCADE,
    sequence INTEGER NOT NULL,
    platform_name TEXT NOT NULL,
    opened_on DATE,
    PRIMARY KEY (station_id, platform_id, line_id)
);

-- Entrances; platforms without published exits (light rail) have one at the platform.
CREATE TABLE IF NOT EXISTS transit_exits (
    station_id TEXT NOT NULL REFERENCES transit_stations (id) ON DELETE CASCADE,
    exit_id TEXT NOT NULL, -- platform ID:exit ID
    name TEXT NOT NULL DEFAULT '',
    location GEOGRAPHY(POINT, 4326) NOT NULL,
    PRIMARY KEY (station_id, exit_id)
);

CREATE INDEX IF NOT EXISTS transit_exits_location_gist_idx ON transit_exits USING GIST (location);

-- Walking-distance proxy: straight-line metres from a sale to the station's nearest exit,
-- for sales located at a door number within 800 m. A sale may be near several stations.
CREATE TABLE IF NOT EXISTS transaction_station_distances (
    station_id TEXT NOT NULL REFERENCES transit_stations (id) ON DELETE CASCADE,
    transaction_id TEXT NOT NULL,
    distance_m REAL NOT NULL,
    PRIMARY KEY (station_id, transaction_id)
);

CREATE INDEX IF NOT EXISTS transaction_station_distances_transaction_idx ON transaction_station_distances (transaction_id);
