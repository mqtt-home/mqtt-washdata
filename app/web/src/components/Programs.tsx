import { Sparkles } from 'lucide-react'
import { PowerChart } from '@/components/PowerChart'
import { Program } from '@/types/dryer'
import { useI18n } from '@/lib/i18n'

interface Props {
  programs: Program[]
}

export function Programs({ programs }: Props) {
  const i = useI18n()
  if (programs.length === 0) {
    return (
      <div className="card">
        <p className="px-6 py-10 text-center text-sm" style={{ color: 'var(--ink-3)' }}>
          {i.t('programs.none')}
        </p>
      </div>
    )
  }

  return (
    <div className="grid gap-4 md:grid-cols-2">
      {programs.map((p) => (
        <section key={p.name} className="card min-w-0">
          <div className="card-h">
            <h3 className="min-w-0 truncate">{p.name}</h3>
            {p.auto && (
              <span title={i.t('programs.auto')} aria-label={i.t('programs.auto')} className="inline-flex" style={{ color: 'var(--ink-3)' }}>
                <Sparkles className="size-4" />
              </span>
            )}
            <span className="meta num">{p.runs === 1 ? i.t('programs.runs1') : i.t('programs.runs', { n: p.runs })}</span>
          </div>
          <div className="card-b space-y-3">
            <PowerChart
              compact
              samples={p.profile.map((power, idx) => ({ t: (idx / (p.profile.length - 1)) * p.medianDurationSec, power }))}
              height={150}
            />
            <div className="stats compact">
              <div className="stat">
                <div className="k">{i.t('typical')}</div>
                <div className="v">{i.dur(p.medianDurationSec)}</div>
                {p.minDurationSec < p.maxDurationSec && (
                  <div className="h">
                    {i.dur(p.minDurationSec)} – {i.dur(p.maxDurationSec)}
                  </div>
                )}
              </div>
              <div className="stat">
                <div className="k">{i.t('energy')}</div>
                <div className="v">{i.energy(p.medianEnergyWh)}</div>
              </div>
              <div className="stat">
                <div className="k">{i.t('peak')}</div>
                <div className="v">{i.watts(p.peakPower)}</div>
              </div>
            </div>
            {p.auto && (
              <p className="text-xs" style={{ color: 'var(--ink-3)' }}>
                {i.t('programs.autoHint')}
              </p>
            )}
          </div>
        </section>
      ))}
    </div>
  )
}
