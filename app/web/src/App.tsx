import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import { NavLink, Navigate, Route, Routes, useNavigate, useParams } from 'react-router-dom'
import { Download, LayoutDashboard, ListTree, Upload, Wind } from 'lucide-react'
import { LiveCard } from '@/components/LiveCard'
import { RunsList } from '@/components/RunsList'
import { RunDetail } from '@/components/RunDetail'
import { Programs } from '@/components/Programs'
import { useLiveStatus } from '@/hooks/useSSE'
import { EXPORT_URL, fetchPrograms, fetchRuns, importData } from '@/lib/api'
import { LangSwitch, useI18n } from '@/lib/i18n'
import { Program, Run } from '@/types/dryer'

export function App() {
  const i18n = useI18n()
  const { t } = i18n
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
      <header className="top-bar">
        <div className="mx-auto flex max-w-3xl flex-wrap items-center gap-x-3 gap-y-2 px-safe py-2.5">
          <div className="order-1 flex min-w-0 flex-1 items-center gap-3">
            <span className="ico">
              <Wind />
            </span>
            <div className="min-w-0">
              <div className="truncate text-[17px] leading-tight font-semibold tracking-tight">{t('title')}</div>
              <div className="flex items-center gap-1.5 text-xs" style={{ color: 'var(--ink-3)' }}>
                {isConnected ? (
                  'washdata'
                ) : (
                  <>
                    <span className="dot bad" /> {t('disconnected')}
                  </>
                )}
              </div>
            </div>
          </div>
          <div className="order-2 sm:order-4">
            <LangSwitch i18n={i18n} />
          </div>
          <div className="order-3 flex w-full items-center gap-1 sm:w-auto">
            <nav className="seg nav-seg">
              <NavItem to="/">
                <LayoutDashboard className="size-4" /> {t('nav.overview')}
              </NavItem>
              <NavItem to="/programs">
                <ListTree className="size-4" /> {t('nav.programs')}
              </NavItem>
            </nav>
            <div className="ml-auto flex">
              <ExportImport onImported={load} />
            </div>
          </div>
        </div>
      </header>

      <main className="mx-auto max-w-3xl space-y-4 px-safe pt-5 pb-[max(1.5rem,env(safe-area-inset-bottom))]">
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
  const { t } = useI18n()
  const fileRef = useRef<HTMLInputElement>(null)

  const onFile = async (file: File | undefined) => {
    if (!file) return
    try {
      const result = await importData(file)
      alert(t('import.done', { n: result.imported, skipped: result.skipped }))
      onImported()
    } catch (e) {
      alert(t('import.failed', { error: e instanceof Error ? e.message : String(e) }))
    } finally {
      if (fileRef.current) fileRef.current.value = ''
    }
  }

  return (
    <>
      <a href={EXPORT_URL} download title={t('export')} aria-label={t('export')} className="icon-btn">
        <Download className="size-[18px]" />
      </a>
      <button title={t('import')} aria-label={t('import')} className="icon-btn" onClick={() => fileRef.current?.click()}>
        <Upload className="size-[18px]" />
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

function NavItem({ to, children }: { to: string; children: ReactNode }) {
  return (
    <NavLink to={to} end className={({ isActive }) => (isActive ? 'active' : '')}>
      {children}
    </NavLink>
  )
}
