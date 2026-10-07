-- Precomputed building category and age for filtering and the analysis pages.
-- Categories are the part of 建物型態 before "(", with every other type as 其他.
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS building_category TEXT GENERATED ALWAYS AS (
    CASE split_part(building_type, '(', 1)
        WHEN '住宅大樓' THEN '住宅大樓'
        WHEN '華廈' THEN '華廈'
        WHEN '公寓' THEN '公寓'
        WHEN '透天厝' THEN '透天厝'
        ELSE '其他'
    END
) STORED;

-- Whole years between completion and sale; NULL when the completion date is unknown.
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS building_age INTEGER GENERATED ALWAYS AS (
    CASE WHEN completed_on IS NULL OR completed_on > transacted_on THEN NULL
         ELSE floor((transacted_on - completed_on) / 365.25)::INTEGER
    END
) STORED;

CREATE INDEX IF NOT EXISTS transactions_building_category_idx ON transactions (building_category, transacted_on);
