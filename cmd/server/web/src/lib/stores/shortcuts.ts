import { browser } from '$app/environment'
import { initDispatcher } from '$lib/shortcuts/dispatcher'
import { DEFAULT_KEYMAP } from '$lib/shortcuts/registry'
import type { KeyMap, ShortcutAction } from '$lib/shortcuts/types'
import { writable } from 'svelte/store'

const STORAGE_KEY = 'damask:keymap'

function loadKeymap(): KeyMap {
  if (!browser) return DEFAULT_KEYMAP
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return DEFAULT_KEYMAP
    const partial = JSON.parse(raw) as Partial<KeyMap>
    return { ...DEFAULT_KEYMAP, ...partial }
  } catch {
    return DEFAULT_KEYMAP
  }
}

export const keymap = writable<KeyMap>(loadKeymap())

keymap.subscribe((map) => {
  if (browser) {
    const overrides: Partial<KeyMap> = {}
    for (const [action, combos] of Object.entries(map)) {
      const key = action as ShortcutAction
      if (JSON.stringify(combos) !== JSON.stringify(DEFAULT_KEYMAP[key])) {
        overrides[key] = combos
      }
    }
    if (Object.keys(overrides).length > 0) {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(overrides))
    } else {
      localStorage.removeItem(STORAGE_KEY)
    }
    initDispatcher(map)
  }
})

export function resetShortcut(action: ShortcutAction): void {
  keymap.update((map) => ({ ...map, [action]: DEFAULT_KEYMAP[action] }))
}

export function resetAllShortcuts(): void {
  keymap.set({ ...DEFAULT_KEYMAP })
}

export function setShortcut(action: ShortcutAction, combos: string[]): void {
  keymap.update((map) => ({ ...map, [action]: combos }))
}
