import { tagApi } from '$lib/api'
import type { DuplicateTagPair, Tag } from '$lib/api/models'

function createTagsManagementStore() {
  let tags = $state<Tag[]>([])
  let loading = $state(false)
  let stale = $state(false)
  let duplicates = $state<DuplicateTagPair[]>([])
  let dismissedPairs = $state<Set<string>>(new Set())

  function pairKey(a: string, b: string) {
    return [a, b].sort().join('\x00')
  }

  async function load() {
    stale = false
    loading = true
    try {
      tags = await tagApi.list()
    } finally {
      loading = false
    }
  }

  function invalidate() {
    stale = true
  }

  async function loadDuplicates() {
    try {
      duplicates = await tagApi.duplicateSuggestions()
    } catch {
      duplicates = []
    }
  }

  async function createTag(
    name: string,
    color?: string | null,
    groupName?: string | null
  ) {
    const tag = await tagApi.create(name, color, groupName)
    tags = [...tags, tag].sort((a, b) => a.name.localeCompare(b.name))
    return tag
  }

  async function patchTag(
    name: string,
    fields: { name?: string; color?: string | null; group_name?: string | null }
  ) {
    // Optimistic update
    const prev = tags.find((t) => t.name === name)
    if (prev) {
      tags = tags.map((t) => (t.name === name ? { ...t, ...fields } : t))
    }
    try {
      const updated = await tagApi.patch(name, fields)
      tags = tags.map((t) =>
        t.name === updated.name || t.name === name ? updated : t
      )
      return updated
    } catch (e) {
      // Revert
      if (prev)
        tags = tags.map((t) => (t.name === (fields.name ?? name) ? prev : t))
      throw e
    }
  }

  async function bulkDelete(names: string[]) {
    // Optimistic
    const removed = tags.filter((t) => names.includes(t.name))
    tags = tags.filter((t) => !names.includes(t.name))
    try {
      return await tagApi.bulkDelete(names)
    } catch (e) {
      tags = [...tags, ...removed].sort((a, b) => a.name.localeCompare(b.name))
      throw e
    }
  }

  async function mergeTags(sources: string[], target: string) {
    const result = await tagApi.merge(sources, target)
    // Remove source rows, update or add target
    tags = tags
      .filter((t) => !sources.includes(t.name))
      .map((t) => (t.name === target ? result.target : t))
    // If target was new, add it
    if (!tags.find((t) => t.name === target)) {
      tags = [...tags, result.target].sort((a, b) =>
        a.name.localeCompare(b.name)
      )
    }
    // Remove resolved duplicate pairs
    duplicates = duplicates.filter(
      (p) => !sources.includes(p.a) && !sources.includes(p.b)
    )
    return result
  }

  function dismissDuplicate(a: string, b: string) {
    dismissedPairs = new Set([...dismissedPairs, pairKey(a, b)])
  }

  const visibleDuplicates = $derived(
    duplicates.filter((p) => !dismissedPairs.has(pairKey(p.a, p.b)))
  )

  return {
    get tags() {
      return tags
    },
    get loading() {
      return loading
    },
    get stale() {
      return stale
    },
    get duplicates() {
      return visibleDuplicates
    },
    get allGroups() {
      const groups = new Set<string>()
      for (const t of tags) if (t.group_name) groups.add(t.group_name)
      return [...groups].sort()
    },
    load,
    invalidate,
    loadDuplicates,
    createTag,
    patchTag,
    bulkDelete,
    mergeTags,
    dismissDuplicate,
  }
}

export const tagsManagementStore = createTagsManagementStore()
