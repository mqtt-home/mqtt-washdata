import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import { NavLink, Navigate, Route, Routes, useNavigate, useParams } from 'react-router-dom'
import { Download, LayoutDashboard, Upload, Waves, WifiOff } from 'lucide-react'
import { ThemeToggle } from '@/components/ThemeToggle'
import { LiveCard } from '@/components/LiveCard'
import { RunsList } from '@/components/RunsList'
import { RunDetail } from '@/components/RunDetail'
import { Programs } from '@/components/Programs'
import { useLiveStatus } from '@/hooks/useSSE'
import { EXPORT_URL, fetchPrograms, fetchRuns, importData } from '@/lib/api'
import { Program, Run } from '@/types/dryer'

export function App() {
  const { status, isConnected } = useLiveStatus()
  const [runs, setRuns] = useState<Run[]>([])
  const [programs, setPrograms] = useState<Program[]>([])
  const prevState = useRef<string | undefined>(undefined)

  const load = useCallback(() => {
    fetchRuns().then(setRuns).catch(() => {})
    fetchPrograms().then(setPrograms).catch(() => {})
  }, [])

  // No polling: history only changes on events we already observe. Load on
  // startup and whenever the SSE stream (re)connects — a reconnect may mean
  // the backend restarted and relearned its programs.
  useEffect(() => {
    load()
  }, [load])

  useEffect(() => {
    if (isConnected) load()
  }, [isConnected, load])

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
              <NavButton to="/">
                <LayoutDashboard className="h-4 w-4" /> Dashboard
              </NavButton>
              <NavButton to="/programs">
                <Waves className="h-4 w-4" /> Programs
              </NavButton>
            </nav>
            <ExportImport onImported={load} />
            <ThemeToggle />
          </div>
        </div>
      </header>

      <main className="mx-auto max-w-3xl space-y-4 px-4 py-6">
        <Routes>
          <Route path="/" element={<Dashboard status={status} connected={isConnected} runs={runs} />} />
          <Route path="/programs" element={<Programs programs={programs} />} />
          <Route path="/runs/:id" element={<RunDetailRoute programs={programs} onChanged={load} />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </main>
    </div>
  )
}

function Dashboard({
  status,
  connected,
  runs,
}: {
  status: ReturnType<typeof useLiveStatus>['status']
  connected: boolean
  runs: Run[]
}) {
  const navigate = useNavigate()
  return (
    <>
      <LiveCard status={status} connected={connected} />
      <RunsList runs={runs} onSelect={(id) => navigate(`/runs/${encodeURIComponent(id)}`)} />
    </>
  )
}

function RunDetailRoute({ programs, onChanged }: { programs: Program[]; onChanged: () => void }) {
  const { id } = useParams()
  const navigate = useNavigate()
  if (!id) return <Navigate to="/" replace />
  return <RunDetail id={id} programs={programs} onBack={() => navigate('/')} onChanged={onChanged} />
}

function ExportImport({ onImported }: { onImported: () => void }) {
  const fileRef = useRef<HTMLInputElement>(null)

  const onFile = async (file: File | undefined) => {
    if (!file) return
    try {
      const result = await importData(file)
      alert(`Imported ${result.imported} run${result.imported === 1 ? '' : 's'} (${result.skipped} skipped).`)
      onImported()
    } catch (e) {
      alert(`Import failed: ${e instanceof Error ? e.message : e}`)
    } finally {
      if (fileRef.current) fileRef.current.value = ''
    }
  }

  const iconButton =
    'rounded p-2 text-[color:var(--color-muted-foreground)] transition-colors hover:text-[color:var(--color-foreground)]'

  return (
    <>
      <a href={EXPORT_URL} download title="Export runs" className={iconButton}>
        <Download className="h-4 w-4" />
      </a>
      <button title="Import runs" className={iconButton} onClick={() => fileRef.current?.click()}>
        <Upload className="h-4 w-4" />
      </button>
      <input
        ref={fileRef}
        type="file"
        accept="application/json,.json"
        className="hidden"
        onChange={(e) => onFile(e.target.files?.[0])}
      />
    </>
  )
}

function NavButton({ to, children }: { to: string; children: ReactNode }) {
  return (
    <NavLink
      to={to}
      end
      className={({ isActive }) =>
        `flex items-center gap-1.5 rounded px-3 py-1.5 text-sm font-medium transition-colors ${
          isActive
            ? 'bg-[color:var(--color-primary)] text-[color:var(--color-primary-foreground)]'
            : 'text-[color:var(--color-muted-foreground)] hover:text-[color:var(--color-foreground)]'
        }`
      }
    >
      {children}
    </NavLink>
  )
}
