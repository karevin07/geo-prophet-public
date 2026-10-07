import { Bar, CartesianGrid, ComposedChart, Line, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import type { MonthlyAnalysis } from "../../lib/api";
import { useLanguage } from "../../lib/i18n";

export default function AnalysisChart({ monthly }: { monthly: MonthlyAnalysis[] }) {
  const { t } = useLanguage();
  const firstProvisional = monthly.findIndex((point) => point.provisional);
  const rows = monthly.map((point, index) => {
    const price = Math.round(point.avgPricePerPing / 1000) / 10;
    return {
      month: point.month,
      count: point.count,
      price: firstProvisional < 0 || index < firstProvisional ? price : undefined,
      provisional: firstProvisional >= 0 && index >= firstProvisional - 1 ? price : undefined,
    };
  });

  return (
    <div className="h-72 w-full sm:h-80" role="img" aria-label={t("analysis.trendDescription")}>
      <ResponsiveContainer width="100%" height="100%">
        <ComposedChart data={rows} margin={{ top: 12, right: 12, bottom: 0, left: -14 }}>
          <CartesianGrid stroke="#e2e8f0" strokeDasharray="3 3" vertical={false} />
          <XAxis dataKey="month" tickFormatter={(value: string) => value.slice(2)} minTickGap={38} fontSize={11} stroke="#94a3b8" tickLine={false} axisLine={false} />
          <YAxis yAxisId="price" domain={["auto", "auto"]} tickFormatter={(value: number) => String(Math.round(value))} fontSize={11} stroke="#94a3b8" tickLine={false} axisLine={false} />
          <YAxis yAxisId="count" orientation="right" fontSize={11} stroke="#94a3b8" tickLine={false} axisLine={false} />
          <Tooltip
            contentStyle={{ border: "1px solid #e2e8f0", borderRadius: 10, fontSize: 12 }}
            formatter={(value, name) => [Number(value).toLocaleString("zh-TW"), name]}
          />
          <Bar yAxisId="count" dataKey="count" name={t("analysis.salesCount")} fill="#d5e9eb" maxBarSize={14} isAnimationActive={false} />
          <Line yAxisId="price" type="monotone" dataKey="price" name={t("analysis.avgUnit")} stroke="#1f6f78" strokeWidth={2.5} dot={false} connectNulls={false} isAnimationActive={false} />
          <Line yAxisId="price" type="monotone" dataKey="provisional" name={t("analysis.provisionalPrice")} stroke="#1f6f78" strokeWidth={2.5} strokeDasharray="5 5" dot={false} isAnimationActive={false} />
        </ComposedChart>
      </ResponsiveContainer>
    </div>
  );
}
