import type { ReactNode } from 'react'
import { Activity, Clock, Gauge, Power, Zap } from 'lucide-react'
import { Card, CardContent } from '@/components/ui/card'
import { LiveStatus } from '@/types/dryer'
import { formatClock, formatDuration, formatEnergy, formatWatts } from '@/lib/utils'

interface Props {
  status: LiveStatus | null
  connected: boolean
}

export function LiveCard({ status, connected }: Props) {
  const running = status?.state === 'running'
  const antiCrease = running && status?.phase === 'anti-crease'
  const progress = status && status.progress >= 0 ? status.progress : null
  const remaining = status && status.remainingSec >= 0 ? status.remainingSec : null

  return (
    <Card className="overflow-hidden">
      <CardContent className="p-6">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <span
              className={`inline-flex h-2.5 w-2.5 rounded-full ${
                running ? 'bg-[color:var(--color-success)] animate-pulse' : 'bg-[color:var(--color-muted-foreground)]'
              }`}
            />
            <h2 className="text-xl font-semibold">{status?.dryerName ?? 'Dryer'}</h2>
          </div>
          <span
            className={`rounded-full px-3 py-1 text-xs font-medium ${
              antiCrease
                ? 'bg-[color:var(--color-warning)]/15 text-[color:var(--color-warning)]'
                : running
                  ? 'bg-[color:var(--color-success)]/15 text-[color:var(--color-success)]'
                  : 'bg-[color:var(--color-muted)] text-[color:var(--color-muted-foreground)]'
            }`}
          >
            {connected ? (antiCrease ? 'Knitterschutz' : running ? 'Running' : 'Idle') : 'Offline'}
          </span>
        </div>

        {running ? (
          <>
            <div className="mt-6 flex items-end gap-3">
              {antiCrease ? (
                <>
                  <div className="text-5xl font-bold">Knitterschutz</div>
                  <div className="pb-1 text-sm text-[color:var(--color-muted-foreground)]">
                    done — laundry can be removed
                  </div>
                </>
              ) : (
                <>
                  <div className="text-5xl font-bold tabular-nums">
                    {remaining !== null ? formatDuration(remaining) : '–'}
                  </div>
                  <div className="pb-1 text-sm text-[color:var(--color-muted-foreground)]">remaining</div>
                </>
              )}
            </div>

            <div className="mt-4 h-2.5 w-full overflow-hidden rounded-full bg-[color:var(--color-muted)]">
              <div
                className="h-full rounded-full bg-[color:var(--color-primary)] transition-all duration-700"
                style={{ width: `${progress !== null ? Math.round(progress * 100) : 0}%` }}
              />
            </div>

            <div className="mt-2 flex items-center justify-between text-sm text-[color:var(--color-muted-foreground)]">
              <span>
                {status?.program ? (
                  <>
                    {status.program}
                    {status.confidence > 0 && (
                      <span className="ml-1 opacity-70">({Math.round(status.confidence * 100)}%)</span>
                    )}
                  </>
                ) : (
                  'Detecting program…'
                )}
              </span>
              {!antiCrease && (
                <span className="flex items-center gap-1">
                  <Clock className="h-3.5 w-3.5" /> ETA {formatClock(status?.eta)}
                </span>
              )}
            </div>

            <div className="mt-6 grid grid-cols-3 gap-3">
              <Metric icon={<Gauge className="h-4 w-4" />} label="Power" value={formatWatts(status?.power ?? 0)} />
              <Metric icon={<Zap className="h-4 w-4" />} label="Energy" value={formatEnergy(status?.energyWh ?? 0)} />
              <Metric
                icon={<Activity className="h-4 w-4" />}
                label="Elapsed"
                value={formatDuration(status?.elapsedSec ?? 0)}
              />
            </div>
          </>
        ) : (
          <div className="mt-8 flex flex-col items-center justify-center gap-3 py-6 text-[color:var(--color-muted-foreground)]">
            <Power className="h-10 w-10 opacity-40" />
            <p>The dryer is not running.</p>
            <p className="text-sm">Current draw: {formatWatts(status?.power ?? 0)}</p>
          </div>
        )}
      </CardContent>
    </Card>
  )
}

function Metric({ icon, label, value }: { icon: ReactNode; label: string; value: string }) {
  return (
    <div className="rounded-lg bg-[color:var(--color-muted)]/50 p-3">
      <div className="flex items-center gap-1.5 text-xs text-[color:var(--color-muted-foreground)]">
        {icon}
        {label}
      </div>
      <div className="mt-1 text-lg font-semibold tabular-nums">{value}</div>
    </div>
  )
}
