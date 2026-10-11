// 数量、金额、时间格式化，与 App 的展示规则一致。
// Quantity, money and time formatting, matching the App's display rules.

export const DAY_MS = 86_400_000;

// PRICE_RE 金额：非负，最多两位小数。
// PRICE_RE matches a non-negative amount with at most two decimals.
export const PRICE_RE = /^\d+(\.\d{1,2})?$/;

export function fmtQty(v: number): string {
  return String(Math.round((Number(v) || 0) * 1000) / 1000);
}

export function fmtQtyUnit(v: number, unit?: string): string {
  return unit ? `${fmtQty(v)} ${unit}` : fmtQty(v);
}

export function fmtMoney(v: number): string {
  const n = Number(v) || 0;
  return (Math.round(n * 100 + (n >= 0 ? 1e-7 : -1e-7)) / 100).toFixed(2);
}

const pad = (n: number) => String(n).padStart(2, '0');

export function fmtTime(ms: number): string {
  const d = new Date(ms);
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

export function fmtDayShort(ms: number): string {
  const d = new Date(ms);
  return `${pad(d.getMonth() + 1)}/${pad(d.getDate())}`;
}

export function daysSince(ms: number): number {
  return Math.floor(Math.max(0, Date.now() - ms) / DAY_MS);
}

// toLocalInput 毫秒时间戳 → datetime-local 输入框的值。
// toLocalInput converts a millisecond timestamp into a datetime-local input value.
export function toLocalInput(ms: number): string {
  const d = new Date(ms);
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

// fileUrl 把存储相对路径转换为图片访问地址。
// fileUrl turns a stored relative path into the image URL.
export function fileUrl(rel: string | null | undefined): string {
  return rel ? '/files/' + rel.split('/').map(encodeURIComponent).join('/') : '';
}

const TAG_PALETTE: [string, string][] = [
  ['#E3F2FD', '#1565C0'], ['#E0F2F1', '#00695C'], ['#FFF3E0', '#E65100'], ['#F3E5F5', '#6A1B9A'],
  ['#E8F5E9', '#2E7D32'], ['#FFEBEE', '#C62828'], ['#E8EAF6', '#283593'], ['#ECEFF1', '#37474F'],
];

// tagColor 按标签名哈希稳定取色，同名标签在任何页面颜色一致。
// tagColor picks a stable color by hashing the tag so the same tag looks the same everywhere.
export function tagColor(tag: string): { bg: string; fg: string } {
  let hash = 0;
  for (const ch of tag) hash = (hash * 31 + (ch.codePointAt(0) ?? 0)) | 0;
  const [bg, fg] = TAG_PALETTE[Math.abs(hash) % TAG_PALETTE.length];
  return { bg, fg };
}

// parseOptions 解析用户输入的候选值，支持逗号、顿号、竖线分隔并去重。
// parseOptions splits user-entered options by commas, Chinese commas or pipes and de-duplicates them.
export function parseOptions(raw: string): string[] {
  return [...new Set(raw.split(/[,，、|]/).map((s) => s.trim()).filter(Boolean))];
}

export function splitTags(value: string): string[] {
  return value.split('|').filter(Boolean);
}
