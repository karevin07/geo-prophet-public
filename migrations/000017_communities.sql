-- Communities (社區): residential buildings completed together. 實價登錄 names only
-- presale projects, so a community is inferred from sales: the door numbers on one road,
-- lane or alley whose 住宅大樓, 華廈 or 公寓 sales share a completion date.

-- clean_address drops spaces and the 里 and 鄰 some addresses carry after the district,
-- and writes 121-1號 as 121之1號.
CREATE OR REPLACE FUNCTION clean_address(address TEXT) RETURNS TEXT
LANGUAGE SQL IMMUTABLE PARALLEL SAFE AS $$
    SELECT regexp_replace(
        regexp_replace(regexp_replace(address, '\s', '', 'g'), '區([^0-9路街道段巷弄號區]{1,3}里)?([0-9]+鄰)?', '區'),
        '([0-9])-([0-9])', '\1之\2', 'g')
$$;

-- door_number returns a cleaned address up to its door number, e.g.
-- 台北市大安區仁愛路四段27巷3號 for 台北市大安區仁愛路四段27巷3號二樓; NULL without a door
-- number or for a land lot (地號).
CREATE OR REPLACE FUNCTION door_number(address TEXT) RETURNS TEXT
LANGUAGE SQL IMMUTABLE PARALLEL SAFE AS $$
    SELECT CASE WHEN address LIKE '%地號%' THEN NULL ELSE substring(public.clean_address(address) FROM '^(.*?[0-9]+號)') END
$$;

-- community_key identifies a community by its city, the street part of the door number
-- and the completion date; NULL for other building types or without either. Functions are
-- schema-qualified so that restoring a dump, which empties search_path, can compute it.
CREATE OR REPLACE FUNCTION community_key(city_code TEXT, address TEXT, building_type TEXT, completed_on DATE) RETURNS TEXT
LANGUAGE SQL IMMUTABLE PARALLEL SAFE AS $$
    SELECT CASE
        WHEN completed_on IS NULL OR split_part(building_type, '(', 1) NOT IN ('住宅大樓', '華廈', '公寓')
            OR public.door_number(address) IS NULL THEN NULL
        ELSE left(md5(city_code || '|' || regexp_replace(public.door_number(address), '[0-9]+(之[0-9]+)?號$', '')
            || '|' || (completed_on - DATE '2000-01-01')::TEXT), 12)
    END
$$;

ALTER TABLE transactions ADD COLUMN IF NOT EXISTS community_key TEXT
    GENERATED ALWAYS AS (public.community_key(city_code, address, building_type, completed_on)) STORED;
CREATE INDEX IF NOT EXISTS transactions_community_idx ON transactions (community_key, transacted_on) WHERE community_key IS NOT NULL;

ALTER TABLE rentals ADD COLUMN IF NOT EXISTS community_key TEXT
    GENERATED ALWAYS AS (public.community_key(city_code, address, building_type, completed_on)) STORED;
CREATE INDEX IF NOT EXISTS rentals_community_idx ON rentals (community_key, leased_on) WHERE community_key IS NOT NULL;

-- Presale projects name communities. Their addresses are often only a road or lane, so
-- they are kept as a door number, if any, and the street before it.
CREATE OR REPLACE FUNCTION address_street(address TEXT) RETURNS TEXT
LANGUAGE SQL IMMUTABLE PARALLEL SAFE AS $$
    SELECT regexp_replace(public.clean_address(address), '[0-9]+(之[0-9]+)?號.*$', '')
$$;

ALTER TABLE presale_projects ADD COLUMN IF NOT EXISTS door_number TEXT GENERATED ALWAYS AS (public.door_number(address)) STORED;
ALTER TABLE presale_projects ADD COLUMN IF NOT EXISTS street TEXT GENERATED ALWAYS AS (public.address_street(address)) STORED;
ALTER TABLE presale_projects ADD COLUMN IF NOT EXISTS first_contract DATE;
ALTER TABLE presale_projects ADD COLUMN IF NOT EXISTS presales INTEGER NOT NULL DEFAULT 0;

-- Same as 000015, also recording each project's first contract and number of presales.
CREATE OR REPLACE FUNCTION refresh_presale_projects(keys TEXT[]) RETURNS VOID LANGUAGE SQL AS $$
    DELETE FROM presale_projects WHERE project_key = ANY (keys);
    INSERT INTO presale_projects (project_key, city_code, district, name, address, location, geocode_precision, first_contract, presales)
    SELECT DISTINCT ON (project_key) project_key, city_code, district, project_name, address, location, geocode_precision,
        MIN(transacted_on) OVER (PARTITION BY project_key), COUNT(*) OVER (PARTITION BY project_key)
    FROM presales
    WHERE project_key = ANY (keys)
    ORDER BY project_key,
        CASE geocode_precision WHEN 'address' THEN 0 WHEN 'nearby' THEN 1 WHEN 'road' THEN 2 WHEN 'district' THEN 3 ELSE 4 END,
        transacted_on DESC, id;
$$;

UPDATE presale_projects p SET first_contract = s.first_contract, presales = s.presales
FROM (SELECT project_key, MIN(transacted_on) AS first_contract, COUNT(*) AS presales FROM presales GROUP BY project_key) s
WHERE s.project_key = p.project_key;

-- One row per community with sales, rebuilt from them.
CREATE TABLE IF NOT EXISTS communities (
    community_key TEXT PRIMARY KEY,
    city_code TEXT NOT NULL,
    district TEXT NOT NULL,
    street TEXT NOT NULL, -- the door numbers without their number, e.g. 台北市中山區敬業三路162巷
    name TEXT NOT NULL DEFAULT '', -- the presale project it was sold as, if known
    completed_on DATE NOT NULL,
    door_numbers TEXT[] NOT NULL, -- e.g. {台北市中山區敬業三路162巷100號, ...}
    category TEXT NOT NULL, -- the most common building category of its sales
    total_floors INTEGER, -- the highest recorded
    sales INTEGER NOT NULL,
    last_sale DATE NOT NULL,
    -- The centre of its sales located to an address, or NULL without any.
    location GEOGRAPHY(POINT, 4326)
);

CREATE INDEX IF NOT EXISTS communities_city_idx ON communities (city_code, district);
CREATE INDEX IF NOT EXISTS communities_location_idx ON communities USING GIST (location);
CREATE INDEX IF NOT EXISTS communities_doors_idx ON communities USING GIN (door_numbers);

-- Names the 住宅大樓 and 華廈 communities of the given areas (city|district) after a
-- presale project of their district whose first contract came at most six years before
-- completion, at one of their door numbers or on their street within 400 m. A project
-- names a community only when each is the other's best match, so a project on a long
-- road does not name every building along it. Presales carry project names from about
-- 2019, so older communities stay unnamed.
CREATE OR REPLACE FUNCTION name_communities(areas TEXT[]) RETURNS VOID LANGUAGE SQL AS $$
    WITH scope AS (
        SELECT * FROM communities WHERE city_code || '|' || district IN (SELECT unnest(areas))
    ), candidates AS (
        SELECT c.community_key, p.name, p.door_number = ANY (c.door_numbers) AS same_door, c.street = p.street AS same_street,
            ST_Distance(p.location, c.location, FALSE) AS distance, p.project_key, p.presales, p.first_contract, c.completed_on, c.sales
        FROM scope c JOIN presale_projects p ON p.city_code = c.city_code AND p.district = c.district
        WHERE c.category IN ('住宅大樓', '華廈') AND p.first_contract IS NOT NULL
          AND (p.door_number = ANY (c.door_numbers)
               OR (p.street ~ '[路街道]' AND left(c.street, length(p.street)) = p.street))
          AND c.completed_on >= p.first_contract AND c.completed_on < p.first_contract + INTERVAL '6 years'
          AND (p.location IS NULL OR c.location IS NULL OR ST_DWithin(p.location, c.location, 400, FALSE))
    ), ranked AS (
        SELECT community_key, name,
            row_number() OVER (PARTITION BY community_key ORDER BY same_door DESC NULLS LAST, same_street DESC, distance NULLS LAST,
                abs(completed_on - (first_contract + 3 * 365)), presales DESC, name) AS community_rank,
            row_number() OVER (PARTITION BY project_key ORDER BY same_door DESC NULLS LAST, same_street DESC, distance NULLS LAST,
                abs(completed_on - (first_contract + 3 * 365)), sales DESC, community_key) AS project_rank
        FROM candidates
    )
    UPDATE communities c SET name = COALESCE(best.name, '')
    FROM scope LEFT JOIN (SELECT community_key, name FROM ranked WHERE community_rank = 1 AND project_rank = 1) best USING (community_key)
    WHERE c.community_key = scope.community_key;
$$;

-- Rebuilds the given communities from their sales; the importer calls it after each upsert
-- with the communities it touched. Keys are joined rather than compared with = ANY, which
-- scans the whole array for every sale.
CREATE OR REPLACE FUNCTION refresh_communities(keys TEXT[]) RETURNS VOID LANGUAGE SQL AS $$
    DELETE FROM communities WHERE community_key IN (SELECT unnest(keys));
    INSERT INTO communities (community_key, city_code, district, street, completed_on, door_numbers, category,
        total_floors, sales, last_sale, location)
    SELECT community_key, MIN(city_code), mode() WITHIN GROUP (ORDER BY district),
        MIN(regexp_replace(door_number(address), '[0-9]+(之[0-9]+)?號$', '')), MIN(completed_on),
        array_agg(DISTINCT door_number(address) ORDER BY door_number(address)),
        mode() WITHIN GROUP (ORDER BY building_category), MAX(total_floors), COUNT(*), MAX(transacted_on),
        ST_Centroid(ST_Collect(location::geometry) FILTER (WHERE geocode_precision IN ('address', 'nearby')))::geography
    FROM transactions
    WHERE community_key IN (SELECT unnest(keys))
    GROUP BY community_key;
    SELECT name_communities(ARRAY(SELECT DISTINCT city_code || '|' || district FROM communities WHERE community_key IN (SELECT unnest(keys))));
$$;

SELECT refresh_communities(ARRAY(SELECT DISTINCT community_key FROM transactions WHERE community_key IS NOT NULL));
ANALYZE communities;
