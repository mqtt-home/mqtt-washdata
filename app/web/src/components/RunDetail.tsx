import { useEffect, useState } from 'react'
import { ArrowLeft, Check, Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { PowerChart } from '@/components/PowerChart'
import { Program, Run } from '@/types/dryer'
import { deleteRun, fetchRun, labelRun } from '@/lib/api'
import { formatDateTime, formatDuration, formatEnergy, formatWatts } from '@/lib/utils'

interface Props {
  id: string
  programs: Program[]
  onBack: () => void
  onChanged: () => void
}

export function RunDetail({ id, programs, onBack, onChanged }: Props) {
  const [run, setRun] = useState<Run | null>(null)
  const [label, setLabel] = useState('')
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    fetchRun(id)
      .then((r) => {
        setRun(r)
        setLabel(r.labeled ? r.program : '')
      })
      .catch(() => setRun(null))
  }, [id])

  const save = async (program: string) => {
    setSaving(true)
    try {
      const updated = await labelRun(id, program)
      setRun(updated)
      setLabel(updated.labeled ? updated.program : '')
      onChanged()
    } finally {
      setSaving(false)
    }
  }

  const remove = async () => {
    if (!confirm('Delete this run? This cannot be undone.')) return
    await deleteRun(id)
    onChanged()
    onBack()
  }

  if (!run) {
    return (
      <div className="space-y-4">
        <Button variant="ghost" size="sm" onClick={onBack}>
          <ArrowLeft className="h-4 w-4" /> Back
        </Button>
        <p className="text-[color:var(--color-muted-foreground)]">Loading…</p>
      </div>
    )
  }

  const suggestions = programs.map((p) => p.name).filter((n) => n && n !== label)

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <Button variant="ghost" size="sm" onClick={onBack}>
          <ArrowLeft className="h-4 w-4" /> Back
        </Button>
        <Button variant="destructive" size="sm" onClick={remove}>
          <Trash2 className="h-4 w-4" /> Delete
        </Button>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>{run.program || 'Unlabeled run'}</CardTitle>
          <p className="text-sm text-[color:var(--color-muted-foreground)]">{formatDateTime(run.start)}</p>
        </CardHeader>
        <CardContent className="space-y-6">
          <PowerChart samples={run.samples ?? []} phases={run.phases} />

          <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
            <Stat label="Duration" value={formatDuration(run.durationSec)} />
            <Stat label="Energy" value={formatEnergy(run.energyWh)} />
            <Stat label="Peak" value={formatWatts(run.peakPower)} />
            <Stat label="Average" value={formatWatts(run.meanPower)} />
          </div>

          {run.programAuto && (
            <p className="text-sm text-[color:var(--color-muted-foreground)]">
              Auto-detected: <span className="font-medium">{run.programAuto}</span>
              {run.confidence > 0 && <> ({Math.round(run.confidence * 100)}% match)</>}
            </p>
          )}

          <div>
            <label className="mb-1.5 block text-sm font-medium">Program label</label>
            <div className="flex flex-wrap gap-2">
              <input
                value={label}
                onChange={(e) => setLabel(e.target.value)}
                placeholder="e.g. Cottons, Synthetics, Eco…"
                className="h-10 flex-1 rounded-md border border-[color:var(--color-input)] bg-[color:var(--color-background)] px-3 text-sm focus:outline-none focus:ring-2 focus:ring-[color:var(--color-ring)]"
              />
              <Button onClick={() => save(label.trim())} disabled={saving || !label.trim()}>
                <Check className="h-4 w-4" /> Save
              </Button>
              {run.labeled && (
                <Button variant="outline" onClick={() => save('')} disabled={saving}>
                  Clear
                </Button>
              )}
            </div>
            {suggestions.length > 0 && (
              <div className="mt-2 flex flex-wrap gap-2">
                {suggestions.map((s) => (
                  <button
                    key={s}
                    onClick={() => save(s)}
                    className="rounded-full border border-[color:var(--color-input)] px-3 py-1 text-xs hover:bg-[color:var(--color-accent)]"
                  >
                    {s}
                  </button>
                ))}
              </div>
            )}
          </div>
        </CardContent>
      </Card>
    </div>
  )
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg bg-[color:var(--color-muted)]/50 p-3">
      <div className="text-xs text-[color:var(--color-muted-foreground)]">{label}</div>
      <div className="mt-1 text-lg font-semibold tabular-nums">{value}</div>
    </div>
  )
}
