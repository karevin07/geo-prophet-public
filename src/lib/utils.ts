import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

/** Replaces {name} placeholders in a translated template. */
export function fill(template: string, values: Record<string, string | number>) {
  return template.replace(/\{(\w+)\}/g, (match, key: string) => key in values ? String(values[key]) : match);
}

/** Formats an ISO date or timestamp as YYYY-MM-DD. */
export function isoDate(value: string) {
  return value.slice(0, 10);
}

/** Every month (YYYY-MM) from from to to, inclusive. */
export function calendarMonths(from: string, to: string): string[] {
  const [year, month] = from.split("-").map(Number);
  const months: string[] = [];
  for (const date = new Date(Date.UTC(year, month - 1, 1)); date.toISOString().slice(0, 7) <= to; date.setUTCMonth(date.getUTCMonth() + 1)) {
    months.push(date.toISOString().slice(0, 7));
  }
  return months;
}
