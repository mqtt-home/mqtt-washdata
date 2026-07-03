import { LiveStatus, Program, Run } from '@/types/dryer'

export const API_BASE = import.meta.env.DEV ? 'http://localhost:8080/api' : '/api'

export async function fetchStatus(): Promise<LiveStatus> {
  const res = await fetch(`${API_BASE}/status`)
  if (!res.ok) throw new Error('Failed to fetch status')
  return res.json()
}

export async function fetchRuns(): Promise<Run[]> {
  const res = await fetch(`${API_BASE}/runs`)
  if (!res.ok) throw new Error('Failed to fetch runs')
  return (await res.json()) ?? []
}

export async function fetchRun(id: string): Promise<Run> {
  const res = await fetch(`${API_BASE}/runs/${encodeURIComponent(id)}`)
  if (!res.ok) throw new Error('Failed to fetch run')
  return res.json()
}

export async function labelRun(id: string, program: string): Promise<Run> {
  const res = await fetch(`${API_BASE}/runs/${encodeURIComponent(id)}/label`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ program }),
  })
  if (!res.ok) throw new Error('Failed to label run')
  return res.json()
}

export async function deleteRun(id: string): Promise<void> {
  const res = await fetch(`${API_BASE}/runs/${encodeURIComponent(id)}`, { method: 'DELETE' })
  if (!res.ok) throw new Error('Failed to delete run')
}

export async function fetchPrograms(): Promise<Program[]> {
  const res = await fetch(`${API_BASE}/programs`)
  if (!res.ok) throw new Error('Failed to fetch programs')
  return (await res.json()) ?? []
}
