import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import { LayoutDashboard, Waves, WifiOff } from 'lucide-react'
import { ThemeToggle } from '@/components/ThemeToggle'
import { LiveCard } from '@/components/LiveCard'
import { RunsList } from '@/components/RunsList'
import { RunDetail } from '@/components/RunDetail'
import { Programs } from '@/components/Programs'
import { useLiveStatus } from '@/hooks/useSSE'
import { fetchPrograms, fetchRuns } from '@/lib/api'
import { Program, Run } from '@/types/dryer'

type View = 'dashboard' | 'programs'

export function App() {
  const { status, isConnected } = useLiveStatus()
  const [view, setView] = useState<View>('dashboard')
  const [selectedRun, setSelectedRun] = useState<string | null>(null)
  const [runs, setRuns] = useState<Run[]>([])
  const [programs, setPrograms] = useState<Program[]>([])
  const prevState = useRef<string | undefined>(undefined)

  const load = useCallback(() => {
    fetchRuns().then(setRuns).catch(() => {})
    fetchPrograms().then(setPrograms).catch(() => {})
  }, [])

  useEffect(() => {
    load()
    const t = setInterval(load, 15000)
    return () => clearInterval(t)
  }, [load])

  // Reload history when a run finishes (running -> idle).
  useEffect(() => {
    if (prevState.current === 'running' && status?.state === 'idle') {
      load()
    }
    prevState.current = status?.state
  }, [status?.state, load])

  return (
    <div className="min-h-screen">
      <header className="sticky top-0 z-10 border-b bg-[color:var(--color-background)]/80 backdrop-blur">
        <div className="mx-auto flex max-w-3xl items-center justify-between px-4 py-3">
          <div className="flex items-center gap-2">
            <Waves className="h-6 w-6 text-[color:var(--color-primary)]" />
            <span className="text-lg font-semibold">washdata</span>
            {!isConnected && (
              <span title="Disconnected">
                <WifiOff className="h-4 w-4 text-[color:var(--color-destructive)]" />
              </span>
            )}
          </div>
          <div className="flex items-center gap-2">
            <nav className="flex rounded-md border p-0.5">
              <NavButton active={view === 'dashboard'} onClick={() => setView('dashboard')}>
                <LayoutDashboard className="h-4 w-4" /> Dashboard
              </NavButton>
              <NavButton active={view === 'programs'} onClick={() => setView('programs')}>
                <Waves className="h-4 w-4" /> Programs
              </NavButton>
            </nav>
            <ThemeToggle />
          </div>
        </div>
      </header>

      <main className="mx-auto max-w-3xl space-y-4 px-4 py-6">
        {selectedRun ? (
          <RunDetail
            id={selectedRun}
            programs={programs}
            onBack={() => setSelectedRun(null)}
            onChanged={load}
          />
        ) : view === 'dashboard' ? (
          <>
            <LiveCard status={status} connected={isConnected} />
            <RunsList runs={runs} onSelect={setSelectedRun} />
          </>
        ) : (
          <Programs programs={programs} />
        )}
      </main>
    </div>
  )
}

function NavButton({
  active,
  onClick,
  children,
}: {
  active: boolean
  onClick: () => void
  children: ReactNode
}) {
  return (
    <button
      onClick={onClick}
      className={`flex items-center gap-1.5 rounded px-3 py-1.5 text-sm font-medium transition-colors ${
        active
          ? 'bg-[color:var(--color-primary)] text-[color:var(--color-primary-foreground)]'
          : 'text-[color:var(--color-muted-foreground)] hover:text-[color:var(--color-foreground)]'
      }`}
    >
      {children}
    </button>
  )
}
