import { Sparkles } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { PowerChart } from '@/components/PowerChart'
import { Program } from '@/types/dryer'
import { formatDuration, formatEnergy, formatWatts } from '@/lib/utils'

interface Props {
  programs: Program[]
}

export function Programs({ programs }: Props) {
  if (programs.length === 0) {
    return (
      <Card>
        <CardContent className="py-10 text-center text-[color:var(--color-muted-foreground)]">
          No programs learned yet. Finish a few dryer runs and label them to train recognition.
        </CardContent>
      </Card>
    )
  }

  return (
    <div className="grid gap-4 md:grid-cols-2">
      {programs.map((p) => (
        <Card key={p.name}>
          <CardHeader>
            <div className="flex items-center justify-between">
              <CardTitle className="flex items-center gap-2">
                {p.name}
                {p.auto && <Sparkles className="h-4 w-4 text-[color:var(--color-warning)]" />}
              </CardTitle>
              <span className="text-sm text-[color:var(--color-muted-foreground)]">
                {p.runs} run{p.runs === 1 ? '' : 's'}
              </span>
            </div>
          </CardHeader>
          <CardContent className="space-y-4">
            <PowerChart samples={p.profile.map((power, i) => ({ t: (i / (p.profile.length - 1)) * p.medianDurationSec, power }))} height={160} />
            <div className="grid grid-cols-3 gap-3">
              <Stat
                label="Typical"
                value={formatDuration(p.medianDurationSec)}
                hint={p.minDurationSec < p.maxDurationSec ? `${formatDuration(p.minDurationSec)} – ${formatDuration(p.maxDurationSec)}` : undefined}
              />
              <Stat label="Energy" value={formatEnergy(p.medianEnergyWh)} />
              <Stat label="Peak" value={formatWatts(p.peakPower)} />
            </div>
            {p.auto && (
              <p className="text-xs text-[color:var(--color-muted-foreground)]">
                Auto-detected cluster — open a matching run and give it a name to train it.
              </p>
            )}
          </CardContent>
        </Card>
      ))}
    </div>
  )
}

function Stat({ label, value, hint }: { label: string; value: string; hint?: string }) {
  return (
    <div className="rounded-lg bg-[color:var(--color-muted)]/50 p-3">
      <div className="text-xs text-[color:var(--color-muted-foreground)]">{label}</div>
      <div className="mt-1 text-base font-semibold tabular-nums">{value}</div>
      {hint && <div className="text-xs text-[color:var(--color-muted-foreground)] tabular-nums">{hint}</div>}
    </div>
  )
}
