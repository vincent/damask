type DesktopBridge = {
  OpenBrowser?: (url: string) => Promise<void> | void
  PickDirectory?: () => Promise<string> | string
}

declare global {
  interface Window {
    go?: {
      main?: {
        App?: DesktopBridge
      }
    }
    runtime?: {
      App?: DesktopBridge
    }
  }
}

function bridge(): DesktopBridge | null {
  if (typeof window === 'undefined') return null
  return window.go?.main?.App ?? window.runtime?.App ?? null
}

export function isDesktopRuntime(): boolean {
  return bridge() !== null
}

export async function openExternalURL(url: string) {
  const api = bridge()
  if (api?.OpenBrowser) {
    await api.OpenBrowser(url)
    return
  }
  window.open(url, '_blank', 'noopener,noreferrer')
}

export async function pickDirectory(): Promise<string | null> {
  const api = bridge()
  if (!api?.PickDirectory) return null
  const result = await api.PickDirectory()
  return result || null
}
