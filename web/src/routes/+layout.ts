import { browser } from '$app/environment'
import { redirect } from '@sveltejs/kit'
import type { LayoutLoad } from './$types'

const PUBLIC_PATHS = ['/login', '/register', '/invite']

export const ssr = false

export const load: LayoutLoad = async ({ url, fetch }) => {
  if (!browser) return {}

  if (PUBLIC_PATHS.some((p) => url.pathname.startsWith(p))) {
    return {}
  }

  try {
    const res = await fetch('http://localhost:8080/api/v1/workspace/me', {
      credentials: 'include',
    })

    if (!res.ok) {
      redirect(303, '/login')
    }

    return await res.json()
  } catch {
    redirect(303, '/login')
  }
}
