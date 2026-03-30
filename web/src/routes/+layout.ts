import { browser } from '$app/environment'
import { redirect } from '@sveltejs/kit'
import type { LayoutLoad } from './$types'
import { workspaceApi, ApiError } from '$lib/api/client'
import type { User, Workspace } from '$lib/api/client'

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

  try {
    return await workspaceApi.me()
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) {
      redirect(303, '/login')
    }
    redirect(303, '/login')
  }
}
