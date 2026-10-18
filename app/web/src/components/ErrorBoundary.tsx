import { Component, type ErrorInfo, type ReactNode } from 'react'

// Without this a single render error unmounts the whole tree and leaves a white page.
export class ErrorBoundary extends Component<{ children: ReactNode }, { error: Error | null }> {
  state = { error: null as Error | null }

  static getDerivedStateFromError(error: Error) {
    return { error }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('UI crashed', error, info.componentStack)
  }

  render() {
    const { error } = this.state
    if (!error) return this.props.children
    const de = document.documentElement.lang !== 'en'
    return (
      <div className="mx-auto max-w-3xl p-6">
        <h1 className="text-lg font-semibold" style={{ color: 'var(--alert)' }}>
          {de ? 'Die Seite ist auf einen Fehler gestoßen' : 'The page hit an error'}
        </h1>
        <pre className="mt-4 overflow-x-auto rounded-md border p-3 text-xs">{String(error.stack ?? error)}</pre>
        <button className="btn mt-4" onClick={() => location.reload()}>
          {de ? 'Neu laden' : 'Reload'}
        </button>
      </div>
    )
  }
}
