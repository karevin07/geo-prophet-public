-- Flag homes bought from the government as special, matching the remarks lvr.specialRemarks
-- now recognises: 向政府機關承購/申購 (e.g. 合宜住宅), units allotted after 區段徵收 (專案配售),
-- 安置住宅 and 眷村改建 homes. They sell well below market, e.g. 2017 北投 文林北路75巷 at
-- 24–28萬/坪, and dragged district medians down. is_special is generated, so it follows.
UPDATE transactions
SET special_reasons = array_append(special_reasons, 'government_sale')
WHERE remarks ~ '向政府|專案配售|安置住宅|眷村改建|國防部配售'
  AND NOT ('government_sale' = ANY (special_reasons));
