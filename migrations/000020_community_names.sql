-- Names more communities after their presale projects. Presale addresses are often an
-- intersection, a block and unit (元富一街A1-15F) or a land lot rather than a door number,
-- so address_street() found no street for many projects. address_streets() finds every
-- road in such an address, and a project's tallest building now has to agree with the
-- community's when the street is the only evidence.

-- address_streets returns the roads of an address with their section, lane and alley, each
-- prefixed by its city and district, e.g. {桃園市中壢區松仁路,桃園市中壢區松勤路} for
-- 桃園市中壢區松仁路與松勤路交叉口; NULL when it names no road, as for a land lot.
CREATE OR REPLACE FUNCTION address_streets(address TEXT) RETURNS TEXT[]
LANGUAGE SQL IMMUTABLE PARALLEL SAFE AS $$
    WITH cleaned AS (
        -- Sections are written 二段 in door numbers, sometimes 2段 in presale addresses;
        -- intersections read 路與街、路和街、路街口 or 路街交叉口.
        SELECT regexp_replace(regexp_replace(
            replace(replace(replace(replace(replace(replace(replace(replace(replace(public.clean_address(address),
                '1段', '一段'), '2段', '二段'), '3段', '三段'), '4段', '四段'), '5段', '五段'),
                '6段', '六段'), '7段', '七段'), '8段', '八段'), '9段', '九段'),
            '交叉路?口|交會處|交口', '', 'g'), '(路|街|道|段|巷|弄)[和口]', '\1與', 'g') AS text
    ), parts AS (
        SELECT substring(text FROM '^(.{2,3}[市縣].{1,3}?[區市鎮鄉])') AS area, text FROM cleaned
    )
    SELECT array_agg(DISTINCT area || road[1] ORDER BY area || road[1])
    FROM parts, regexp_matches(substr(text, length(area) + 1),
        '([^0-9A-Za-z與及、,，旁段巷弄號地樓之路街\-]+(?:路|街|大道)(?:[一二三四五六七八九十]段)?(?:[0-9]+巷)?(?:[0-9]+弄)?)', 'g') AS road
    WHERE area IS NOT NULL
$$;

ALTER TABLE presale_projects ADD COLUMN IF NOT EXISTS streets TEXT[] GENERATED ALWAYS AS (public.address_streets(address)) STORED;
ALTER TABLE presale_projects ADD COLUMN IF NOT EXISTS total_floors INTEGER; -- its tallest building

-- Same as 000017, also recording the project's tallest building.
CREATE OR REPLACE FUNCTION refresh_presale_projects(keys TEXT[]) RETURNS VOID LANGUAGE SQL AS $$
    DELETE FROM presale_projects WHERE project_key = ANY (keys);
    INSERT INTO presale_projects (project_key, city_code, district, name, address, location, geocode_precision, first_contract, presales, total_floors)
    SELECT DISTINCT ON (project_key) project_key, city_code, district, project_name, address, location, geocode_precision,
        MIN(transacted_on) OVER (PARTITION BY project_key), COUNT(*) OVER (PARTITION BY project_key),
        MAX(total_floors) OVER (PARTITION BY project_key)
    FROM presales
    WHERE project_key = ANY (keys)
    ORDER BY project_key,
        CASE geocode_precision WHEN 'address' THEN 0 WHEN 'nearby' THEN 1 WHEN 'road' THEN 2 WHEN 'district' THEN 3 ELSE 4 END,
        transacted_on DESC, id;
$$;

UPDATE presale_projects p SET total_floors = s.total_floors
FROM (SELECT project_key, MAX(total_floors) AS total_floors FROM presales GROUP BY project_key) s
WHERE s.project_key = p.project_key AND p.total_floors IS DISTINCT FROM s.total_floors;

-- Names the 住宅大樓 and 華廈 communities of the given areas (city|district) after a
-- presale project of their district that had its first contract at most six years before
-- they were completed and lies within 400 m (1.5 km of the middle of its road when only its
-- road is known). The evidence, strongest first:
--   3  the project's door number is one of the community's;
--   2  the community is on one of the project's streets (within 2 floors of its height);
--   1  the community is on one of its roads, in another lane (within 1 floor);
--   0  no street in common, but at most 150 m from its door number and exactly as tall.
-- Ties go to the closest height, then the closest door number, then the completion nearest
-- three years after the first contract.
-- A project names a community only when each is the other's best match, so a project on a
-- long road does not name every building along it. It also names the other communities
-- completed the same day within 150 m of that one whose own best match it is: one complex
-- with doors on several streets. Presales carry project names from about 2019, so older
-- communities stay unnamed.
CREATE OR REPLACE FUNCTION name_communities(areas TEXT[]) RETURNS VOID LANGUAGE SQL AS $$
    WITH scope AS (
        SELECT * FROM communities WHERE city_code || '|' || district IN (SELECT unnest(areas))
    ), candidates AS (
        SELECT c.community_key, c.completed_on, c.location AS community_location, c.sales, p.project_key, p.name, p.presales,
            CASE WHEN p.door_number = ANY (c.door_numbers) THEN 3
                 WHEN EXISTS (SELECT 1 FROM unnest(p.streets) street WHERE left(c.street, length(street)) = street) THEN 2
                 WHEN EXISTS (SELECT 1 FROM unnest(p.streets) street
                              WHERE regexp_replace(street, '[0-9]+巷.*$', '') = regexp_replace(c.street, '[0-9]+巷.*$', '')) THEN 1
                 ELSE 0 END AS evidence,
            abs(c.total_floors - p.total_floors) AS floor_gap,
            -- Only door numbers place a project; a road is placed at its middle.
            CASE WHEN p.geocode_precision IN ('address', 'nearby') THEN ST_Distance(p.location, c.location, FALSE) END AS distance,
            abs(c.completed_on - (p.first_contract + 3 * 365)) AS timing
        FROM scope c JOIN presale_projects p ON p.city_code = c.city_code AND p.district = c.district
        WHERE c.category IN ('住宅大樓', '華廈') AND p.first_contract IS NOT NULL
          AND c.completed_on >= p.first_contract AND c.completed_on < p.first_contract + INTERVAL '6 years'
          AND (c.location IS NULL OR p.location IS NULL OR p.geocode_precision NOT IN ('address', 'nearby', 'road')
               OR ST_DWithin(p.location, c.location, CASE p.geocode_precision WHEN 'road' THEN 1500 ELSE 400 END, FALSE))
    ), plausible AS (
        SELECT * FROM candidates
        WHERE evidence = 3
           OR (evidence = 2 AND (floor_gap <= 2 OR (floor_gap IS NULL AND distance IS NOT NULL)))
           OR (evidence = 1 AND floor_gap <= 1)
           OR (evidence = 0 AND floor_gap = 0 AND distance <= 150)
    ), ranked AS (
        SELECT *,
            row_number() OVER (PARTITION BY community_key ORDER BY evidence DESC, floor_gap NULLS LAST, distance NULLS LAST,
                timing, presales DESC, name) AS community_rank,
            row_number() OVER (PARTITION BY project_key ORDER BY evidence DESC, floor_gap NULLS LAST, distance NULLS LAST,
                timing, sales DESC, community_key) AS project_rank
        FROM plausible
    ), best AS (
        SELECT * FROM ranked WHERE community_rank = 1 AND project_rank = 1
    ), named AS (
        SELECT community_key, name FROM best
        UNION ALL
        SELECT sibling.community_key, sibling.name
        FROM ranked sibling JOIN best ON best.project_key = sibling.project_key
        WHERE sibling.community_rank = 1 AND sibling.project_rank > 1 AND sibling.evidence >= 1
          AND sibling.completed_on = best.completed_on
          AND ST_DWithin(sibling.community_location, best.community_location, 150, FALSE)
    )
    UPDATE communities c SET name = COALESCE(named.name, '')
    FROM scope LEFT JOIN named USING (community_key)
    WHERE c.community_key = scope.community_key AND c.name IS DISTINCT FROM COALESCE(named.name, '');
$$;

SELECT name_communities(ARRAY(SELECT DISTINCT city_code || '|' || district FROM communities));
