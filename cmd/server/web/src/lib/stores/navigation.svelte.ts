let activeProjectId = $state<string | null>(null)
let activeFolderId = $state<string | null>(null)
let activeCollectionId = $state<string | null>(null)

export const navigationStore = {
  get activeProjectId() { return activeProjectId },
  get activeFolderId() { return activeFolderId },
  get activeCollectionId() { return activeCollectionId },

  selectProject(id: string | null) {
    activeProjectId = id
    activeFolderId = null
    activeCollectionId = null
  },

  selectFolder(folderId: string | null) {
    activeFolderId = folderId
    activeCollectionId = null
  },

  selectCollection(id: string | null) {
    activeCollectionId = id
    activeProjectId = null
    activeFolderId = null
  },

  clear() {
    activeProjectId = null
    activeFolderId = null
    activeCollectionId = null
  },
}
