import { type ClassValue, clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function formatDuration(sec: number): string {
  if (sec < 0 || !isFinite(sec)) return '–'
  const s = Math.round(sec)
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const rs = s % 60
  if (h > 0) return `${h}h ${m}m`
  if (m > 0) return `${m}m ${rs}s`
  return `${rs}s`
}

export function formatWatts(w: number): string {
  if (w >= 1000) return `${(w / 1000).toFixed(2)} kW`
  return `${Math.round(w)} W`
}

export function formatEnergy(wh: number): string {
  if (wh >= 1000) return `${(wh / 1000).toFixed(2)} kWh`
  return `${Math.round(wh)} Wh`
}

export function formatClock(iso?: string): string {
  if (!iso) return '–'
  const d = new Date(iso)
  return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}

export function formatDateTime(iso?: string): string {
  if (!iso) return '–'
  const d = new Date(iso)
  return d.toLocaleString([], {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}
