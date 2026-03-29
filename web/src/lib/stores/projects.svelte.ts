import { projectApi, type Project } from '$lib/api/client'
import { navigationStore } from './navigation.svelte'

let projects = $state<Project[]>([])

const totalAssetCount = $derived(projects.reduce((sum, p) => sum + p.asset_count, 0))
const activeProjectName = $derived(
  navigationStore.activeProjectId
    ? (projects.find((p) => p.id === navigationStore.activeProjectId)?.name ?? 'Folder')
    : null,
)

export const projectsStore = {
  get projects() { return projects },
  get totalAssetCount() { return totalAssetCount },
  get activeProjectName() { return activeProjectName },

  async load() {
    try {
      projects = await projectApi.list()
    } catch {
      // silently ignore — 401 redirect handled by api client
    }
  },

  async create(data: { name: string; color: string }) {
    await projectApi.create(data)
    await projectsStore.load()
  },

  async update(id: string, data: { name: string }) {
    await projectApi.update(id, data)
    await projectsStore.load()
  },

  async delete(id: string) {
    await projectApi.delete(id)
    if (navigationStore.activeProjectId === id) navigationStore.clear()
    await projectsStore.load()
  },
}
