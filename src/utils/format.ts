export function formatMoney(v: number | null | undefined) {
  const n = typeof v === 'number' && Number.isFinite(v) ? v : 0
  return new Intl.NumberFormat('zh-CN', { style: 'currency', currency: 'CNY', maximumFractionDigits: 2 }).format(n)
}

export function formatPct(v: number | null | undefined) {
  const n = typeof v === 'number' && Number.isFinite(v) ? v : 0
  return `${Math.round(n * 100)}%`
}

export function clamp01(v: number) {
  if (v < 0) return 0
  if (v > 1) return 1
  return v
}

