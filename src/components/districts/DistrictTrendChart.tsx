import { CartesianGrid, Legend, Line, LineChart, ReferenceArea, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import type { DistrictTrend } from "../../lib/api";
import { useLanguage } from "../../lib/i18n";
import { DISTRICT_COLORS } from "./colors";
import { calendarMonths, districtChartRows, type DistrictMetric } from "./series";

interface Props {
  districts: string[];
  trends: DistrictTrend[];
  metric: DistrictMetric;
  from: string;
  to: string;
  yoyThrough: string;
}

export default function DistrictTrendChart({ districts, trends, metric, from, to, yoyThrough }: Props) {
  const { t } = useLanguage();
  const months = calendarMonths(from, to);
  const rows = districtChartRows(trends, districts, metric, months);
  const provisional = yoyThrough ? months.find((month) => month > yoyThrough) : undefined;

  return (
    <div className="h-72 w-full sm:h-80" role="img" aria-label={t("districts.chartDescription")}>
      <ResponsiveContainer width="100%" height="100%">
        <LineChart data={rows} margin={{ top: 12, right: 18, bottom: 0, left: -18 }}>
          <CartesianGrid stroke="#e2e8f0" strokeDasharray="3 3" vertical={false} />
          <XAxis dataKey="month" tickFormatter={(month: string) => month.slice(2)} minTickGap={28} fontSize={11} stroke="#94a3b8" tickLine={false} axisLine={false} />
          <YAxis fontSize={11} stroke="#94a3b8" tickLine={false} axisLine={false} tickFormatter={(value: number) => String(Math.round(value))} />
          <Tooltip formatter={(value, name) => [`${Number(value).toLocaleString("zh-TW", { maximumFractionDigits: metric === "price" ? 1 : 0 })} ${metric === "price" ? t("chart.unit") : t("analysis.salesCount")}`, name]} />
          <Legend verticalAlign="bottom" height={28} />
          {provisional && <ReferenceArea x1={provisional} x2={months.at(-1)}
            shape={(area) => <rect x={area.x} y={area.y} width={area.width} height={area.height} fill="#fef3c7" fillOpacity={0.5} />} />}
          {districts.map((district, index) => <Line key={district} type="monotone" dataKey={district} name={district} stroke={DISTRICT_COLORS[index]} strokeWidth={2.5} dot={false} connectNulls={false} isAnimationActive={false} />)}
        </LineChart>
      </ResponsiveContainer>
    </div>
  );
}
