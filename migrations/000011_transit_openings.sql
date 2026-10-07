-- Opening dates of lines and of platforms that opened after their line. TDX does not
-- publish them; they were kept in api/internal/transit/openings.go and are now edited in
-- the admin console. Lines and platforms without a row opened before 2015, when
-- 實價登錄 sales start.
CREATE TABLE IF NOT EXISTS transit_openings (
    target_kind TEXT NOT NULL CHECK (target_kind IN ('line', 'platform')),
    target_id TEXT NOT NULL, -- a line ID such as NTMC-Y or a platform ID such as TYMC-A22
    opened_on DATE NOT NULL,
    source TEXT NOT NULL DEFAULT '', -- where the date comes from, usually a URL
    updated_by TEXT NOT NULL DEFAULT 'migration', -- Tailscale login of the last editor
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (target_kind, target_id)
);

INSERT INTO transit_openings (target_kind, target_id, opened_on, source) VALUES
    ('line', 'TYMC-A', '2017-03-02', 'https://www.ctee.com.tw/news/20170226700027-439901'),
    ('line', 'NTDLRT-V', '2018-12-23', 'https://www.cna.com.tw/news/firstnews/201812230188.aspx'),
    ('line', 'NTMC-Y', '2020-01-31', 'https://zh.wikipedia.org/zh-tw/環狀線_(臺北捷運)'),
    ('line', 'NTALRT-K', '2023-02-10', 'https://www.ntmetro.com.tw/basic/?mode=detail&node=578'),
    ('line', 'NTMC-LB', '2026-06-30', 'https://www.dorts.ntpc.gov.tw/news/indexInfo/xVR2eBJzdJ36'),
    ('platform', 'TRTC-BL01', '2015-07-06', 'https://zh.wikipedia.org/zh-tw/頂埔站'),
    ('platform', 'TRA-1050', '2015-12-23', 'https://zh.wikipedia.org/zh-tw/南樹林車站'),
    ('platform', 'THSR-0990', '2016-07-01', 'https://news.pts.org.tw/article/324933'),
    ('platform', 'NTDLRT-V26', '2020-11-15', 'https://www.cna.com.tw/news/firstnews/202011140019.aspx'),
    ('platform', 'NTDLRT-V27', '2020-11-15', 'https://www.cna.com.tw/news/firstnews/202011140019.aspx'),
    ('platform', 'NTDLRT-V28', '2020-11-15', 'https://www.cna.com.tw/news/firstnews/202011140019.aspx'),
    ('platform', 'TYMC-A22', '2023-07-31', 'https://www.motc.gov.tw/ch/app/data/view?module=news&id=14&serno=03fcdf67-3c92-46b9-b79b-cc98b983dc04'),
    ('platform', 'TRA-1075', '2024-11-30', 'https://www.cna.com.tw/news/ahel/202411300095.aspx')
ON CONFLICT DO NOTHING;

-- Copies transit_openings onto the stored network. The importer calls it after replacing
-- stations and the admin console after each edit, so the rule lives only here:
--   a line opened on its row's date, or before 2015 without one;
--   a platform opened on its own row's date, else on the earliest of its lines' dates, or
--   before 2015 if any of its lines did;
--   a station opened when its first platform did, or before 2015 if any platform did.
CREATE OR REPLACE FUNCTION refresh_transit_openings() RETURNS VOID LANGUAGE SQL AS $$
    UPDATE transit_lines l SET opened_on = (
        SELECT o.opened_on FROM transit_openings o WHERE o.target_kind = 'line' AND o.target_id = l.id);

    WITH platforms AS (
        SELECT sl.platform_id,
            CASE WHEN bool_or(o.opened_on IS NULL) THEN NULL ELSE MIN(o.opened_on) END AS lines_opened
        FROM transit_station_lines sl
        LEFT JOIN transit_openings o ON o.target_kind = 'line' AND o.target_id = sl.line_id
        GROUP BY sl.platform_id)
    UPDATE transit_station_lines sl SET opened_on = COALESCE(own.opened_on, platforms.lines_opened)
    FROM platforms
    LEFT JOIN transit_openings own ON own.target_kind = 'platform' AND own.target_id = platforms.platform_id
    WHERE sl.platform_id = platforms.platform_id;

    UPDATE transit_stations s SET opened_on = (
        SELECT CASE WHEN bool_or(sl.opened_on IS NULL) THEN NULL ELSE MIN(sl.opened_on) END
        FROM transit_station_lines sl WHERE sl.station_id = s.id);
$$;

SELECT refresh_transit_openings();
