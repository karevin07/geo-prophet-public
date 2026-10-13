-- A community's last sale ignores sales dated after their registration could have been
-- made (invalid_date, see 000009): 桃園市中壢區松仁路223號, registered in 110S2 as sold on
-- 2101-03-03, made its community's last sale 2101-03-03. Same as 000017 otherwise; a
-- community whose sales are all misdated keeps their latest date.
CREATE OR REPLACE FUNCTION refresh_communities(keys TEXT[]) RETURNS VOID LANGUAGE SQL AS $$
    DELETE FROM communities WHERE community_key IN (SELECT unnest(keys));
    INSERT INTO communities (community_key, city_code, district, street, completed_on, door_numbers, category,
        total_floors, sales, last_sale, location)
    SELECT community_key, MIN(city_code), mode() WITHIN GROUP (ORDER BY district),
        MIN(regexp_replace(door_number(address), '[0-9]+(之[0-9]+)?號$', '')), MIN(completed_on),
        array_agg(DISTINCT door_number(address) ORDER BY door_number(address)),
        mode() WITHIN GROUP (ORDER BY building_category), MAX(total_floors), COUNT(*),
        COALESCE(MAX(transacted_on) FILTER (WHERE NOT ('invalid_date' = ANY (special_reasons))), MAX(transacted_on)),
        ST_Centroid(ST_Collect(location::geometry) FILTER (WHERE geocode_precision IN ('address', 'nearby')))::geography
    FROM transactions
    WHERE community_key IN (SELECT unnest(keys))
    GROUP BY community_key;
    SELECT name_communities(ARRAY(SELECT DISTINCT city_code || '|' || district FROM communities WHERE community_key IN (SELECT unnest(keys))));
$$;

SELECT refresh_communities(ARRAY(
    SELECT DISTINCT community_key FROM transactions
    WHERE community_key IS NOT NULL AND 'invalid_date' = ANY (special_reasons)));
