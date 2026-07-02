import { useCallback, useEffect, useRef, useState } from 'react'
import { LiveStatus } from '@/types/dryer'
import { API_BASE } from '@/lib/api'

interface SSEReturn {
  status: LiveStatus | null
  isConnected: boolean
}

// useLiveStatus subscribes to the /events SSE stream and returns the latest
// live dryer status, auto-reconnecting on error.
export function useLiveStatus(): SSEReturn {
  const [status, setStatus] = useState<LiveStatus | null>(null)
  const [isConnected, setIsConnected] = useState(false)
  const esRef = useRef<EventSource | null>(null)
  const reconnectRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const connect = useCallback(() => {
    esRef.current?.close()
    if (reconnectRef.current) clearTimeout(reconnectRef.current)

    const es = new EventSource(`${API_BASE}/events`)
    esRef.current = es

    es.onopen = () => setIsConnected(true)
    es.onmessage = (event) => {
      try {
        setStatus(JSON.parse(event.data) as LiveStatus)
      } catch {
        /* ignore keep-alive / malformed frames */
      }
    }
    es.onerror = () => {
      setIsConnected(false)
      es.close()
      reconnectRef.current = setTimeout(connect, 3000)
    }
  }, [])

  useEffect(() => {
    connect()
    return () => {
      esRef.current?.close()
      if (reconnectRef.current) clearTimeout(reconnectRef.current)
    }
  }, [connect])

  return { status, isConnected }
}
