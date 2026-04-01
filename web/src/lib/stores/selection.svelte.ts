import type { Asset } from '$lib/api'

let selectedIds = $state(new Set<string>())
let lastSelectedIndex = $state(-1)

export const selectionStore = {
  get selectedIds() { return selectedIds },
  get lastSelectedIndex() { return lastSelectedIndex },

  handleCardClick(
    asset: Asset,
    index: number,
    assets: Asset[],
    event: MouseEvent,
    isEditor: boolean,
  ): boolean {
    if (!isEditor) return false

    if (event.shiftKey && lastSelectedIndex >= 0 && lastSelectedIndex !== index) {
      const next = new Set(selectedIds)
      const lo = Math.min(lastSelectedIndex, index)
      const hi = Math.max(lastSelectedIndex, index)
      for (let i = lo; i <= hi; i++) next.add(assets[i].id)
      selectedIds = next
      return true
    }

    if (event.shiftKey || selectedIds.size > 0) {
      const next = new Set(selectedIds)
      if (next.has(asset.id)) {
        next.delete(asset.id)
      } else {
        next.add(asset.id)
        lastSelectedIndex = index
      }
      selectedIds = next
      return true
    }

    return false
  },

  remove(id: string) {
    const next = new Set(selectedIds)
    next.delete(id)
    selectedIds = next
    if (selectedIds.size === 0) lastSelectedIndex = -1
  },

  clear() {
    selectedIds = new Set()
    lastSelectedIndex = -1
  },
}
