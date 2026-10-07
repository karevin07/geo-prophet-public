import { CartesianGrid, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { useLanguage } from "../../lib/i18n";
import { formatRent, type RentRow } from "../../lib/rent";
import { cn } from "../../lib/utils";

export const CITY_COLOR = "#1f6f78";
export const DISTRICT_COLOR = "#e8704f";

interface RentTrendChartProps {
  rows: RentRow[];
  /** Legend name of the city line, and of the district line when one is drawn. */
  cityLabel: string;
  districtLabel?: string;
  className?: string;
}

/** Monthly median rent per ping; provisional months are dashed. */
export default function RentTrendChart({ rows, cityLabel, districtLabel, className }: RentTrendChartProps) {
  const { t } = useLanguage();
  if (!rows.some((row) => row.city !== null || row.cityTail !== null)) {
    return <div className={cn("flex items-center justify-center rounded-lg border border-dashed border-slate-200 text-xs text-slate-400", className)}>{t("chart.empty")}</div>;
  }
  // One tick per January: the axis shows years.
  const ticks = rows.map((row) => row.month).filter((month) => month.endsWith("-01"));
  const names: Record<string, string> = { city: cityLabel, cityTail: cityLabel, district: districtLabel ?? "", districtTail: districtLabel ?? "" };
  return (
    <div className={cn("w-full", className)}>
      <ResponsiveContainer width="100%" height="100%">
        <LineChart data={rows} margin={{ top: 8, right: 8, left: -12, bottom: 0 }}>
          <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" vertical={false} />
          <XAxis dataKey="month" ticks={ticks} interval={0} stroke="#94a3b8" fontSize={10} tickLine={false} axisLine={false} tickFormatter={(month: string) => month.slice(0, 4)} />
          <YAxis stroke="#94a3b8" fontSize={10} tickLine={false} axisLine={false} domain={["auto", "auto"]} tickFormatter={(value: number) => formatRent(value)} />
          <Tooltip
            contentStyle={{ background: "#fff", border: "1px solid #e2e8f0", borderRadius: 8, fontSize: 12, boxShadow: "0 4px 16px rgba(15,23,42,0.08)" }}
            labelStyle={{ color: "#64748b", marginBottom: 2 }}
            formatter={(value, name, item) => {
              const key = String(name);
              // The tail repeats the last settled month; show each month once.
              if (key.endsWith("Tail") && item.payload[key.replace("Tail", "")] != null) return [null, null];
              const count = key.startsWith("city") ? item.payload.cityCount : item.payload.districtCount;
              return [`${formatRent(Number(value))} ${t("rents.unit.perPing")}（${count} ${t("rents.unit.leases")}）`, names[key]];
            }}
          />
          <Line type="monotone" dataKey="city" stroke={CITY_COLOR} strokeWidth={2} dot={false} connectNulls isAnimationActive={false} />
          <Line type="monotone" dataKey="cityTail" stroke={CITY_COLOR} strokeWidth={2} strokeDasharray="4 4" dot={false} connectNulls isAnimationActive={false} />
          {districtLabel && <Line type="monotone" dataKey="district" stroke={DISTRICT_COLOR} strokeWidth={2} dot={false} connectNulls isAnimationActive={false} />}
          {districtLabel && <Line type="monotone" dataKey="districtTail" stroke={DISTRICT_COLOR} strokeWidth={2} strokeDasharray="4 4" dot={false} connectNulls isAnimationActive={false} />}
        </LineChart>
      </ResponsiveContainer>
    </div>
  );
}
