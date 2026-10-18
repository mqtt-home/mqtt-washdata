import { useEffect, useState } from 'react'
import { ArrowLeft, Check, Trash2 } from 'lucide-react'
import { PowerChart } from '@/components/PowerChart'
import { Program, Run } from '@/types/dryer'
import { deleteRun, fetchRun, labelRun } from '@/lib/api'
import { useI18n } from '@/lib/i18n'

interface Props {
  id: string
  programs: Program[]
  onBack: () => void
  onChanged: () => void
}

export function RunDetail({ id, programs, onBack, onChanged }: Props) {
  const i = useI18n()
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
    if (!confirm(i.t('delete.confirm'))) return
    await deleteRun(id)
    onChanged()
    onBack()
  }

  const back = (
    <button className="btn" onClick={onBack}>
      <ArrowLeft className="size-4" /> {i.t('back')}
    </button>
  )

  if (!run) {
    return (
      <div className="space-y-4">
        {back}
        <p style={{ color: 'var(--ink-3)' }}>{i.t('loading')}</p>
      </div>
    )
  }

  const suggestions = programs.map((p) => p.name).filter((n) => n && n !== label)

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        {back}
        <button className="btn danger" onClick={remove}>
          <Trash2 className="size-4" /> {i.t('delete')}
        </button>
      </div>

      <section className="card">
        <div className="card-h flex-wrap">
          <h3 className="min-w-0 text-[17px]! truncate">{run.program || i.t('run.unlabeled')}</h3>
          <span className="meta num">{i.dateTime(run.start)}</span>
        </div>
        <div className="card-b space-y-5">
          <PowerChart samples={run.samples ?? []} phases={run.phases} />

          <div className="stats grid-cols-2! sm:grid-cols-4!">
            <Stat label={i.t('duration')} value={i.dur(run.durationSec)} />
            <Stat label={i.t('energy')} value={i.energy(run.energyWh)} />
            <Stat label={i.t('peak')} value={i.watts(run.peakPower)} />
            <Stat label={i.t('average')} value={i.watts(run.meanPower)} />
          </div>

          {run.programAuto && (
            <p className="text-sm" style={{ color: 'var(--ink-3)' }}>
              {i.t('run.auto', { program: '' })}
              <span className="font-medium" style={{ color: 'var(--ink)' }}>
                {run.programAuto}
              </span>
              {run.confidence > 0 && <> · {i.t('run.match', { pct: i.num(Math.round(run.confidence * 100)) })}</>}
            </p>
          )}

          <div>
            <label htmlFor="label" className="kicker mb-2 block">
              {i.t('label.title')}
            </label>
            <div className="flex flex-wrap gap-2">
              <div className="field min-w-0 flex-1 basis-56">
                <input
                  id="label"
                  value={label}
                  onChange={(e) => setLabel(e.target.value)}
                  placeholder={i.t('label.placeholder')}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' && label.trim()) save(label.trim())
                  }}
                />
              </div>
              <button className="btn primary" onClick={() => save(label.trim())} disabled={saving || !label.trim()}>
                <Check className="size-4" /> {i.t('save')}
              </button>
              {run.labeled && (
                <button className="btn" onClick={() => save('')} disabled={saving}>
                  {i.t('clear')}
                </button>
              )}
            </div>
            {suggestions.length > 0 && (
              <div className="mt-3 flex flex-wrap gap-2">
                {suggestions.map((s) => (
                  <button key={s} onClick={() => save(s)} className="chip">
                    {s}
                  </button>
                ))}
              </div>
            )}
          </div>
        </div>
      </section>
    </div>
  )
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="stat">
      <div className="k">{label}</div>
      <div className="v">{value}</div>
    </div>
  )
}
