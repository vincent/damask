import { browser } from '$app/environment'
import { replaceState } from '$app/navigation'
import { redirect } from '@sveltejs/kit'
import type { LayoutLoad } from './$types'
import { workspaceApi } from '$lib/api'
import type { User, Workspace } from '$lib/api'

const PUBLIC_PATHS = ['/login', '/register', '/invite', '/s/']

export const ssr = false

export const load: LayoutLoad = async ({ url, fetch }): Promise<{
  user?: User
  workspace?: Workspace
  role?: string
}> => {
  if (!browser) return {}

  if (PUBLIC_PATHS.some((p) => url.pathname.startsWith(p))) {
    return {}
  }

  workspaceApi.useFetch(fetch)

  let result: { user?: User; workspace?: Workspace; role?: string } = {}
  try {
    result = await workspaceApi.me()
  } catch {
    redirect(303, '/login')
  }

  const wsParam = url.searchParams.get('ws')
  if (wsParam && wsParam !== result.workspace?.id) {
    try {
      const switched = await workspaceApi.switch(wsParam)
      result = { user: result.user, workspace: switched.workspace, role: switched.role }
      const clean = new URL(url)
      clean.searchParams.delete('ws')
      replaceState(clean.pathname + clean.search, {})
    } catch {
      // not a member of that workspace — stay in current one
    }
  }

  return result
}
