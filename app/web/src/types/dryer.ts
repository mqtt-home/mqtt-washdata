export type DryerState = 'idle' | 'running'

export interface LiveStatus {
  state: DryerState
  dryerName: string
  power: number
  program?: string
  confidence: number
  energyWh: number
  elapsedSec: number
  remainingSec: number // -1 when unknown
  progress: number // 0..1, or -1 when unknown
  eta?: string
  runId?: string
  updatedAt: string
}

export interface PowerSample {
  t: number // seconds since run start
  power: number
}

export interface Run {
  id: string
  start: string
  end?: string
  finished: boolean
  durationSec: number
  energyWh: number
  peakPower: number
  meanPower: number
  program: string
  programAuto: string
  confidence: number
  labeled: boolean
  samples?: PowerSample[]
}

export interface Program {
  name: string
  auto: boolean
  runs: number
  medianDurationSec: number
  minDurationSec: number
  maxDurationSec: number
  medianEnergyWh: number
  peakPower: number
  profile: number[]
}
