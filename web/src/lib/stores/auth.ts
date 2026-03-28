import { writable } from 'svelte/store'
import type { User, Workspace } from '$lib/api/client'

export interface AuthState {
  user: User | null
  workspace: Workspace | null
  role: string | null
  isAuthenticated: boolean
}

const initial: AuthState = {
  user: null,
  workspace: null,
  role: null,
  isAuthenticated: false,
}

function createAuthStore() {
  const { subscribe, set, update } = writable<AuthState>(initial)

  return {
    subscribe,
    set,
    login(user: User, workspace: Workspace, role: string) {
      set({ user, workspace, role, isAuthenticated: true })
    },
    logout() {
      set(initial)
    },
    update,
  }
}

export const authStore = createAuthStore()
