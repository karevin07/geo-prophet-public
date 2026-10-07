-- The feed sorts by total price with an id tie-break for keyset paging; without an index
-- every such page scanned the whole table.
CREATE INDEX IF NOT EXISTS transactions_total_price_idx ON transactions (total_price DESC, id DESC);

-- Refresh planner statistics. 000005 rewrote the table with new columns and, until
-- autovacuum caught up, the planner badly underestimated rows and chose slow plans.
ANALYZE transactions;
