import type { CSSProperties, ReactNode } from 'react'
import { Activity, Gauge, Power, Wind, Zap } from 'lucide-react'
import { LiveStatus } from '@/types/dryer'
import { useI18n } from '@/lib/i18n'

interface Props {
  status: LiveStatus | null
  connected: boolean
}

export function LiveCard({ status, connected }: Props) {
  const i = useI18n()
  const running = status?.state === 'running'
  const antiCrease = running && status?.phase === 'anti-crease'
  const cooling = running && status?.phase === 'cooling'
  const progress = status && status.progress >= 0 ? status.progress : null
  const remaining = status && status.remainingSec >= 0 ? status.remainingSec : null

  // Colour only ever means state.
  const [stateClass, stateLabel] = !connected
    ? ['bad', i.t('state.offline')]
    : antiCrease
      ? ['warn', i.t('state.antiCrease')]
      : cooling
        ? ['info', i.t('state.cooling')]
        : running
          ? ['ok', i.t('state.running')]
          : ['', i.t('state.idle')]

  const pct = antiCrease ? 100 : progress !== null ? Math.round(progress * 100) : 0

  return (
    <section className="card">
      <div className="card-h">
        <span className="ico sm" style={{ '--c': running ? 'var(--accent)' : 'var(--idle)' } as CSSProperties}>
          <Wind />
        </span>
        <h3 className="min-w-0 truncate">{status?.dryerName || i.t('title')}</h3>
        <span className={`state ${stateClass} ml-auto`}>
          <span className={`dot ${running && connected ? 'live' : ''} ${stateClass ? '' : 'idle'}`} />
          {stateLabel}
        </span>
      </div>

      <div className="card-b">
        {running ? (
          <>
            <div className="mt-2 flex items-center gap-5">
              <div
                className={`pring ${antiCrease ? 'warn' : ''}`}
                style={{ '--p': pct } as CSSProperties}
                role="progressbar"
                aria-label={i.t('progress')}
                aria-valuenow={progress !== null || antiCrease ? pct : undefined}
                aria-valuemin={0}
                aria-valuemax={100}
              >
                <span className="pct">{progress !== null || antiCrease ? `${pct} %` : '–'}</span>
              </div>
              <div className="min-w-0">
                {antiCrease ? (
                  <>
                    <div className="kicker">{i.t('state.antiCrease')}</div>
                    <div className="mt-1.5 text-[17px] leading-snug font-semibold">{i.t('antiCrease.hint')}</div>
                  </>
                ) : (
                  <>
                    <div className="kicker">{i.t('remaining')}</div>
                    <div className="big mt-1.5">{remaining !== null ? i.dur(remaining) : '–'}</div>
                    {status?.eta && (
                      <div className="num mt-1.5 text-sm" style={{ color: 'var(--ink-3)' }}>
                        {i.t('eta.at', { time: i.clock(status.eta) })}
                      </div>
                    )}
                  </>
                )}
              </div>
            </div>

            <div className="mt-4 flex items-baseline gap-2 text-sm" style={{ color: 'var(--ink-3)' }}>
              <span>{i.t('program')}</span>
              <span className="font-medium" style={{ color: 'var(--ink)' }}>
                {status?.program || i.t('detecting')}
              </span>
              {status?.program && status.confidence > 0 && (
                <span className="num">· {i.num(Math.round(status.confidence * 100))} %</span>
              )}
            </div>

            <div className="stats mt-4">
              <Metric icon={<Gauge />} label={i.t('power')} value={i.watts(status?.power ?? 0)} />
              <Metric icon={<Zap />} label={i.t('energy')} value={i.energy(status?.energyWh ?? 0)} />
              <Metric icon={<Activity />} label={i.t('elapsed')} value={i.dur(status?.elapsedSec ?? 0)} />
            </div>
          </>
        ) : (
          <div className="flex items-center gap-4 py-4">
            <span className="ico" style={{ '--c': 'var(--idle)' } as CSSProperties}>
              <Power />
            </span>
            <div>
              <div className="font-medium">{i.t('idle.text')}</div>
              <div className="num text-sm" style={{ color: 'var(--ink-3)' }}>
                {i.t('idle.draw', { power: i.watts(status?.power ?? 0) })}
              </div>
            </div>
          </div>
        )}
      </div>
    </section>
  )
}

function Metric({ icon, label, value }: { icon: ReactNode; label: string; value: string }) {
  return (
    <div className="stat">
      <div className="k">
        {icon}
        {label}
      </div>
      <div className="v">{value}</div>
    </div>
  )
}
