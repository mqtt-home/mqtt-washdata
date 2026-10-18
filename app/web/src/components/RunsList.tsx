import { ChevronRight, Tag } from 'lucide-react'
import { Run } from '@/types/dryer'
import { useI18n } from '@/lib/i18n'

interface Props {
  runs: Run[]
  onSelect: (id: string) => void
}

export function RunsList({ runs, onSelect }: Props) {
  const i = useI18n()
  return (
    <section className="card">
      <div className="card-h">
        <h3>{i.t('runs')}</h3>
        {runs.length > 0 && <span className="meta num">{i.t('runs.count', { n: runs.length })}</span>}
      </div>
      <div className="card-b pt-1.5 pb-2">
        {runs.length === 0 ? (
          <p className="py-6 text-center text-sm" style={{ color: 'var(--ink-3)' }}>
            {i.t('runs.none')}
          </p>
        ) : (
          <ul className="list">
            {runs.map((run) => (
              <li key={run.id}>
                <button onClick={() => onSelect(run.id)} className="row-link">
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2 font-medium">
                      <span className="truncate" style={run.program ? undefined : { color: 'var(--ink-3)' }}>
                        {run.program || i.t('unlabeled')}
                      </span>
                      {run.labeled && (
                        <Tag className="size-3.5 shrink-0" style={{ color: 'var(--ink-4)' }} aria-label={i.t('labeled')} />
                      )}
                    </div>
                    <div className="num text-[13px]" style={{ color: 'var(--ink-3)' }}>
                      {i.dateTime(run.start)}
                    </div>
                  </div>
                  <div className="num flex shrink-0 items-center gap-4 text-[13px]" style={{ color: 'var(--ink-2)' }}>
                    <span>{i.dur(run.durationSec)}</span>
                    <span className="hidden w-20 text-right sm:inline" style={{ color: 'var(--ink-3)' }}>
                      {i.energy(run.energyWh)}
                    </span>
                    <ChevronRight className="size-4" style={{ color: 'var(--ink-4)' }} />
                  </div>
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>
    </section>
  )
}
