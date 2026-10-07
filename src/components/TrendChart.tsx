import { useMemo } from "react";
import { Area, AreaChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { useLanguage } from "../lib/i18n";
import type { TrendPoint } from "../lib/api";

interface TrendChartProps {
  /** Provisional months (registrations still arriving) are drawn dashed. */
  data: TrendPoint[];
}

export default function TrendChart({ data }: TrendChartProps) {
  const { t } = useLanguage();

  // Two series share one point at the boundary so the dashed tail joins the solid line.
  const chartData = useMemo(() => {
    const firstProvisional = data.findIndex((point) => point.provisional);
    return data.map((point, index) => {
      const price = Math.round(point.avgPricePerPing / 1000) / 10;
      const isProvisional = firstProvisional >= 0 && index >= firstProvisional;
      const joinsProvisional = firstProvisional > 0 && index === firstProvisional - 1;
      return {
        // 2026-09 → 26/09
        name: `${point.month.slice(2, 4)}/${point.month.slice(5, 7)}`,
        price: isProvisional ? undefined : price,
        provisional: isProvisional || joinsProvisional ? price : undefined,
        count: point.count,
      };
    });
  }, [data]);

  if (chartData.length === 0) {
    return (
      <div className="flex h-36 items-center justify-center rounded-lg border border-dashed border-slate-200 text-xs text-slate-400">
        {t("chart.empty")}
      </div>
    );
  }

  return (
    <div className="h-36 w-full">
      <ResponsiveContainer width="100%" height="100%">
        <AreaChart data={chartData} margin={{ top: 8, right: 4, left: -18, bottom: 0 }}>
          <defs>
            <linearGradient id="trend-fill" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor="#1f6f78" stopOpacity={0.18} />
              <stop offset="100%" stopColor="#1f6f78" stopOpacity={0} />
            </linearGradient>
          </defs>
          <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" vertical={false} />
          <XAxis dataKey="name" stroke="#94a3b8" fontSize={10} tickLine={false} axisLine={false} interval="preserveStartEnd" minTickGap={24} />
          <YAxis stroke="#94a3b8" fontSize={10} tickLine={false} axisLine={false} domain={["dataMin - 5", "dataMax + 5"]} tickFormatter={(value: number) => `${Math.round(value)}`} />
          <Tooltip
            contentStyle={{ background: "#fff", border: "1px solid #e2e8f0", borderRadius: 8, fontSize: 12, boxShadow: "0 4px 16px rgba(15,23,42,0.08)" }}
            labelStyle={{ color: "#64748b", marginBottom: 2 }}
            formatter={(value, name, item) => name === "provisional" && item.payload.price !== undefined
              ? [null, null]
              : [`${value} ${t("chart.unit")}（${item.payload.count} ${t("chart.count")}）`, t("chart.price")]}
          />
          <Area type="linear" dataKey="price" stroke="#1f6f78" strokeWidth={2} fill="url(#trend-fill)" dot={false} activeDot={{ r: 4, fill: "#1f6f78", stroke: "#fff", strokeWidth: 2 }} connectNulls={false} />
          <Area type="linear" dataKey="provisional" stroke="#1f6f78" strokeWidth={2} strokeDasharray="4 4" fill="url(#trend-fill)" fillOpacity={0.5} dot={false} activeDot={{ r: 4, fill: "#fff", stroke: "#1f6f78", strokeWidth: 2 }} />
        </AreaChart>
      </ResponsiveContainer>
    </div>
  );
}
