/** 格式化小工具。 */

export function formatBytes(bytes?: number | null): string {
  const n = Number(bytes ?? 0);
  if (!Number.isFinite(n) || n <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let value = n;
  let i = 0;
  while (value >= 1024 && i < units.length - 1) {
    value /= 1024;
    i += 1;
  }
  const digits = value >= 100 || i === 0 ? 0 : 1;
  return `${value.toFixed(digits)} ${units[i]}`;
}

/** 分钟 → "12 小时 30 分" */
export function formatPlaytime(minutes?: number | null): string {
  const m = Math.max(0, Math.round(Number(minutes ?? 0)));
  if (m === 0) return "未记录";
  const hours = Math.floor(m / 60);
  const rest = m % 60;
  if (hours === 0) return `${rest} 分钟`;
  if (rest === 0) return `${hours} 小时`;
  return `${hours} 小时 ${rest} 分`;
}

/** 小时数（保留一位小数），用于统计页。 */
export function formatHours(minutes?: number | null): string {
  const m = Math.max(0, Number(minutes ?? 0));
  const hours = m / 60;
  if (hours === 0) return "0";
  return hours >= 100 ? Math.round(hours).toString() : hours.toFixed(1);
}

/** 任意时间字符串 → YYYY-MM-DD，解析不了就原样返回。 */
export function formatDate(value?: string | null): string {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  const y = date.getFullYear();
  const m = `${date.getMonth() + 1}`.padStart(2, "0");
  const d = `${date.getDate()}`.padStart(2, "0");
  return `${y}-${m}-${d}`;
}

/** 相对时间：刚刚 / 3 小时前 / 5 天前 / 2024-01-02 */
export function timeAgo(value?: string | null): string {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  const diff = Date.now() - date.getTime();
  if (diff < 0) return formatDate(value);
  const minute = 60_000;
  const hour = 60 * minute;
  const day = 24 * hour;
  if (diff < minute) return "刚刚";
  if (diff < hour) return `${Math.floor(diff / minute)} 分钟前`;
  if (diff < day) return `${Math.floor(diff / hour)} 小时前`;
  if (diff < 30 * day) return `${Math.floor(diff / day)} 天前`;
  return formatDate(value);
}

/** 只留出版年份。 */
export function yearOf(releaseDate?: string | null, fallback?: number | null): string {
  if (releaseDate) {
    const match = /(\d{4})/.exec(releaseDate);
    if (match) return match[1];
  }
  if (fallback && fallback > 0) return String(fallback);
  return "";
}

/** "a, b、c" → ["a","b","c"] */
export function splitList(value?: string | null): string[] {
  if (!value) return [];
  return value
    .split(/[,，;；/、|]+/)
    .map((s) => s.trim())
    .filter(Boolean);
}

/** 数组 → "a, b, c" */
export function joinList(values?: string[] | null): string {
  if (!values || values.length === 0) return "";
  return values.map((s) => s.trim()).filter(Boolean).join(", ");
}

export function truncate(text: string, max: number): string {
  const s = (text ?? "").trim();
  if (s.length <= max) return s;
  return `${s.slice(0, max).trimEnd()}…`;
}

/** 评分 0-10 → 星星个数（四舍五入到半星）。 */
export function starCount(rating?: number | null): number {
  const r = Number(rating ?? 0);
  if (!Number.isFinite(r) || r <= 0) return 0;
  return Math.round(r);
}
