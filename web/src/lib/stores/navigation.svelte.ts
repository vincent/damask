let activeProjectId = $state<string | null>(null)
let activeFolderId = $state<string | null>(null)

export const navigationStore = {
  get activeProjectId() { return activeProjectId },
  get activeFolderId() { return activeFolderId },

  selectProject(id: string | null) {
    activeProjectId = id
    activeFolderId = null
  },

  selectFolder(folderId: string | null) {
    activeFolderId = folderId
  },

  clear() {
    activeProjectId = null
    activeFolderId = null
  },
}
