-- Door-number coordinates (門牌位置坐標資料) per city, replaced wholesale on each import.
CREATE TABLE IF NOT EXISTS address_points (
    city_code TEXT NOT NULL,
    district_code TEXT NOT NULL,
    road TEXT NOT NULL,
    lane TEXT NOT NULL DEFAULT '',
    alley TEXT NOT NULL DEFAULT '',
    number INTEGER NOT NULL CHECK (number > 0),
    sub_number INTEGER NOT NULL DEFAULT 0 CHECK (sub_number >= 0),
    lat DOUBLE PRECISION NOT NULL,
    lng DOUBLE PRECISION NOT NULL,
    PRIMARY KEY (city_code, district_code, road, lane, alley, number, sub_number)
);
