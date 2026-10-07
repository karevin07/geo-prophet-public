-- Flag sales dated after they were first imported: a sale is registered after it happens,
-- so such a date is a typo in the source (e.g. 2101-03-03). The importer flags new ones
-- against the season's registration window (lvr.MarkInvalidDates); this backfills rows
-- imported before that check existed. is_special is generated, so it follows.
UPDATE transactions
SET special_reasons = array_append(special_reasons, 'invalid_date')
WHERE transacted_on > created_at::DATE
  AND NOT ('invalid_date' = ANY (special_reasons));
