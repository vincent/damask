import type { User, Workspace } from '$lib/api'

export interface AuthState {
  user: User | null
  workspace: Workspace | null
  role: string | null
  isAuthenticated: boolean
}

const state = $state<AuthState>({
  user: null,
  workspace: null,
  role: null,
  isAuthenticated: false,
})

export const authStore = {
  get user() { return state.user },
  get workspace() { return state.workspace },
  get role() { return state.role },
  get isAuthenticated() { return state.isAuthenticated },

  login(user: User, workspace: Workspace, role: string) {
    state.user = user
    state.workspace = workspace
    state.role = role
    state.isAuthenticated = true
  },

  switchWorkspace(workspace: Workspace, role: string) {
    state.workspace = workspace
    state.role = role
  },

  patchWorkspace(patch: Partial<Workspace>) {
    if (state.workspace) state.workspace = { ...state.workspace, ...patch }
  },

  logout() {
    state.user = null
    state.workspace = null
    state.role = null
    state.isAuthenticated = false
  },
}
