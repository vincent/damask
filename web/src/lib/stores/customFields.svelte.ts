import { fieldDefinitionApi } from '$lib/api/client'
import type { FieldDefinition, FieldScope } from '$lib/api/models'

class CustomFieldsStore {
  assetFields = $state<FieldDefinition[]>([])
  projectFields = $state<FieldDefinition[]>([])
  loading = $state(false)

  async load(scope: FieldScope) {
    this.loading = true
    try {
      const defs = await fieldDefinitionApi.list(scope)
      if (scope === 'asset') {
        this.assetFields = defs
      } else {
        this.projectFields = defs
      }
    } finally {
      this.loading = false
    }
  }

  async loadBoth() {
    this.loading = true
    try {
      const [asset, project] = await Promise.all([
        fieldDefinitionApi.list('asset'),
        fieldDefinitionApi.list('project'),
      ])
      this.assetFields = asset
      this.projectFields = project
    } finally {
      this.loading = false
    }
  }

  fields(scope: FieldScope): FieldDefinition[] {
    return scope === 'asset' ? this.assetFields : this.projectFields
  }

  async reorder(scope: FieldScope, ordered: FieldDefinition[]) {
    const entries = ordered.map((f, i) => ({ id: f.id, position: i }))
    // Optimistic update
    if (scope === 'asset') {
      this.assetFields = ordered
    } else {
      this.projectFields = ordered
    }
    await fieldDefinitionApi.reorder(entries)
  }

  upsertLocal(scope: FieldScope, def: FieldDefinition) {
    const list = scope === 'asset' ? this.assetFields : this.projectFields
    const idx = list.findIndex((f) => f.id === def.id)
    const updated = idx >= 0 ? list.with(idx, def) : [...list, def]
    if (scope === 'asset') {
      this.assetFields = updated
    } else {
      this.projectFields = updated
    }
  }

  removeLocal(scope: FieldScope, id: string) {
    if (scope === 'asset') {
      this.assetFields = this.assetFields.filter((f) => f.id !== id)
    } else {
      this.projectFields = this.projectFields.filter((f) => f.id !== id)
    }
  }
}

export const customFieldsStore = new CustomFieldsStore()
