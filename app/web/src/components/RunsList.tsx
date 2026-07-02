import { ChevronRight, Tag } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Run } from '@/types/dryer'
import { formatDateTime, formatDuration, formatEnergy } from '@/lib/utils'

interface Props {
  runs: Run[]
  onSelect: (id: string) => void
}

export function RunsList({ runs, onSelect }: Props) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Recent runs</CardTitle>
      </CardHeader>
      <CardContent>
        {runs.length === 0 ? (
          <p className="py-6 text-center text-sm text-[color:var(--color-muted-foreground)]">
            No runs recorded yet.
          </p>
        ) : (
          <ul className="divide-y">
            {runs.map((run) => (
              <li key={run.id}>
                <button
                  onClick={() => onSelect(run.id)}
                  className="flex w-full items-center justify-between gap-3 py-3 text-left hover:opacity-80"
                >
                  <div className="min-w-0">
                    <div className="flex items-center gap-2 font-medium">
                      {run.program || 'Unlabeled'}
                      {run.labeled && <Tag className="h-3.5 w-3.5 text-[color:var(--color-primary)]" />}
                    </div>
                    <div className="text-sm text-[color:var(--color-muted-foreground)]">
                      {formatDateTime(run.start)}
                    </div>
                  </div>
                  <div className="flex items-center gap-4 text-sm text-[color:var(--color-muted-foreground)]">
                    <span className="tabular-nums">{formatDuration(run.durationSec)}</span>
                    <span className="hidden tabular-nums sm:inline">{formatEnergy(run.energyWh)}</span>
                    <ChevronRight className="h-4 w-4" />
                  </div>
                </button>
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  )
}
