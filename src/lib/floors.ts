const DIGITS: Record<string, number> = { 零: 0, 〇: 0, 一: 1, 二: 2, 兩: 2, 三: 3, 四: 4, 五: 5, 六: 6, 七: 7, 八: 8, 九: 9 };

/** The first floor of a 移轉層次 such as "二十三層,陽台" → 23, or null for basements and whole buildings. */
export function floorNumber(label: string): number | null {
  const first = label.split(/[,，]/)[0].replace(/^地上/, "");
  const match = first.match(/^(.+)層$/);
  if (!match) return null;
  if (/^\d+$/.test(match[1])) return Number(match[1]) || null;
  let total = 0;
  let digit = -1;
  for (const character of match[1]) {
    if (character in DIGITS) {
      digit = DIGITS[character];
    } else if (character === "十" || character === "百") {
      total += (digit < 0 ? 1 : digit) * (character === "十" ? 10 : 100);
      digit = -1;
    } else {
      return null;
    }
  }
  if (digit > 0) total += digit;
  return total > 0 ? total : null;
}
