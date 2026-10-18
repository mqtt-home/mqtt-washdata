import { Area, AreaChart, CartesianGrid, ReferenceLine, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'
import { PhaseSpan, PowerSample } from '@/types/dryer'
import { useI18n } from '@/lib/i18n'

interface Props {
  samples: PowerSample[]
  phases?: PhaseSpan[]
  height?: number
  /** Compact chart for program cards: fewer ticks. */
  compact?: boolean
}

export function PowerChart({ samples, phases, height = 260, compact }: Props) {
  const i = useI18n()
  const data = samples.map((s) => ({ min: +(s.t / 60).toFixed(2), power: Math.round(s.power) }))
  // Phase boundaries (the initial phase starting at 0 needs no line).
  const marks = (phases ?? []).filter((p) => p.startSec > 0)
  const endSec = samples.length ? samples[samples.length - 1].t : 0
  const gradId = compact ? 'powerFillC' : 'powerFill'

  return (
    <ResponsiveContainer width="100%" height={height}>
      <AreaChart data={data} margin={{ top: 8, right: 8, left: 0, bottom: 0 }}>
        <defs>
          <linearGradient id={gradId} x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor="var(--accent)" stopOpacity={0.28} />
            <stop offset="100%" stopColor="var(--accent)" stopOpacity={0.02} />
          </linearGradient>
        </defs>
        <CartesianGrid vertical={false} stroke="var(--line-2)" />
        <XAxis
          dataKey="min"
          type="number"
          domain={[0, 'dataMax']}
          tickFormatter={(v) => `${Math.round(v)} min`}
          stroke="var(--ink-4)"
          tick={{ fill: 'var(--ink-3)' }}
          tickLine={false}
          axisLine={{ stroke: 'var(--line-2)' }}
          fontSize={11.5}
          tickCount={compact ? 4 : 6}
          allowDecimals={false}
          interval="preserveStart"
          minTickGap={18}
        />
        <YAxis
          tickFormatter={(v) => `${i.num(v)} W`}
          tick={{ fill: 'var(--ink-3)' }}
          tickLine={false}
          axisLine={false}
          fontSize={11.5}
          width={compact ? 50 : 58}
          tickCount={compact ? 3 : 5}
        />
        <Tooltip
          contentStyle={{
            background: 'var(--surface)',
            border: '1px solid var(--line-2)',
            borderRadius: 10,
            boxShadow: 'var(--elevation-2)',
            color: 'var(--ink)',
            fontSize: 12.5,
          }}
          labelStyle={{ color: 'var(--ink-3)' }}
          cursor={{ stroke: 'var(--line-2)' }}
          labelFormatter={(v) => `${i.num(Number(v), 1)} min`}
          formatter={(v) => [`${i.num(Number(v))} W`, i.t('power')]}
        />
        <Area
          type="monotone"
          dataKey="power"
          stroke="var(--accent)"
          strokeWidth={1.75}
          fill={`url(#${gradId})`}
          isAnimationActive={false}
        />
        {marks.map((p) => (
          <ReferenceLine
            key={p.startSec}
            x={+(p.startSec / 60).toFixed(2)}
            stroke="var(--ink-4)"
            strokeDasharray="4 3"
            label={{
              value: i.phase(p.phase),
              // Near the right edge the label would overflow the chart —
              // anchor it on the other side of the line there.
              position: p.startSec > 0.75 * endSec ? 'insideTopRight' : 'insideTopLeft',
              fill: 'var(--ink-3)',
              fontSize: 11,
            }}
          />
        ))}
      </AreaChart>
    </ResponsiveContainer>
  )
}
