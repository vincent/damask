import { browser } from '$app/environment'
import type { HealthResponse } from '$lib/api'
import { apiFetch } from '$lib/api/client'
import { writable } from 'svelte/store'

export type ServerState = 'connecting' | 'ok' | 'degraded' | 'offline'

export interface ServerStatus {
  state: ServerState
  latencyMs: number | null
  version: string | null
  setupRequired: boolean
}

const initialState: ServerStatus = {
  state: 'connecting',
  latencyMs: null,
  version: null,
  setupRequired: false,
}

const { subscribe, set, update } = writable<ServerStatus>(initialState)

let timer: ReturnType<typeof setTimeout> | null = null
let started = false
let firstSuccess = false
let consecutiveOffline = 0
let inFlight = false

async function poll(force = false) {
  if (!browser || inFlight) return
  if (!force && document.visibilityState === 'hidden') return

  inFlight = true
  const startedAt = performance.now()

  try {
    const data = await apiFetch<HealthResponse>('/healthz')
    const latencyMs = Math.round(performance.now() - startedAt)
    firstSuccess = true
    consecutiveOffline = 0
    set({
      state: latencyMs > 1000 ? 'degraded' : 'ok',
      latencyMs,
      version: data.version,
      setupRequired: data.setupRequired,
    })
  } catch {
    consecutiveOffline += 1
    update((current) => ({
      state: !firstSuccess && consecutiveOffline < 3 ? 'connecting' : 'offline',
      latencyMs: null,
      version: current.version,
      setupRequired: current.setupRequired,
    }))
  } finally {
    inFlight = false
    schedule()
  }
}

function schedule() {
  if (!browser) return
  if (timer) clearTimeout(timer)
  if (document.visibilityState === 'hidden') return
  timer = setTimeout(() => {
    void poll()
  }, 5000)
}

function handleVisibilityChange() {
  if (document.visibilityState === 'visible') {
    void poll(true)
    return
  }
  if (timer) clearTimeout(timer)
}

function start() {
  if (!browser || started) return
  started = true
  document.addEventListener('visibilitychange', handleVisibilityChange)
  void poll(true)
}

if (browser) {
  start()
}

export const serverStatus = {
  subscribe,
}

export function reconnect() {
  void poll(true)
}
