import {
  Area,
  AreaChart,
  CartesianGrid,
  ReferenceLine,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import { PhaseSpan, PowerSample } from '@/types/dryer'

const PHASE_LABELS: Record<string, string> = {
  drying: 'Drying',
  cooling: 'Cooling',
  'anti-crease': 'Knitterschutz',
}

interface Props {
  samples: PowerSample[]
  phases?: PhaseSpan[]
  height?: number
}

export function PowerChart({ samples, phases, height = 260 }: Props) {
  const data = samples.map((s) => ({ min: +(s.t / 60).toFixed(2), power: Math.round(s.power) }))
  // Phase boundaries (the initial phase starting at 0 needs no line).
  const marks = (phases ?? []).filter((p) => p.startSec > 0)

  return (
    <ResponsiveContainer width="100%" height={height}>
      <AreaChart data={data} margin={{ top: 8, right: 12, left: 0, bottom: 4 }}>
        <defs>
          <linearGradient id="powerFill" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor="var(--color-primary)" stopOpacity={0.5} />
            <stop offset="100%" stopColor="var(--color-primary)" stopOpacity={0.04} />
          </linearGradient>
        </defs>
        <CartesianGrid strokeDasharray="3 3" stroke="var(--color-border)" />
        <XAxis
          dataKey="min"
          type="number"
          domain={[0, 'dataMax']}
          tickFormatter={(v) => `${Math.round(v)}m`}
          stroke="var(--color-muted-foreground)"
          fontSize={12}
        />
        <YAxis
          tickFormatter={(v) => `${v}W`}
          stroke="var(--color-muted-foreground)"
          fontSize={12}
          width={52}
        />
        <Tooltip
          contentStyle={{
            background: 'var(--color-popover)',
            border: '1px solid var(--color-border)',
            borderRadius: 8,
            color: 'var(--color-popover-foreground)',
          }}
          labelFormatter={(v) => `${Number(v).toFixed(1)} min`}
          formatter={(v: number) => [`${v} W`, 'Power']}
        />
        <Area
          type="monotone"
          dataKey="power"
          stroke="var(--color-primary)"
          strokeWidth={2}
          fill="url(#powerFill)"
          isAnimationActive={false}
        />
        {marks.map((p) => (
          <ReferenceLine
            key={p.startSec}
            x={+(p.startSec / 60).toFixed(2)}
            stroke="var(--color-warning)"
            strokeDasharray="4 3"
            label={{
              value: PHASE_LABELS[p.phase] ?? p.phase,
              position: 'insideTopLeft',
              fill: 'var(--color-warning)',
              fontSize: 11,
            }}
          />
        ))}
      </AreaChart>
    </ResponsiveContainer>
  )
}
