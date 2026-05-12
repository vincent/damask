import { assetFieldApi } from '$lib/api'
import type { AssetFieldValue } from '$lib/api/models'
import { m } from '$lib/paraglide/messages'
import { customFieldsStore } from '$lib/stores/customFields.svelte'
import type { Command } from './types'

function userFields(fields: AssetFieldValue[]) {
  return fields.filter((field) => !field.source || field.source === 'user')
}

export class SetAssetField implements Command {
  constructor(
    private assetId: string,
    private fieldId: string,
    private fieldName: string,
    private before: string | number | boolean | null,
    private after: string | number | boolean
  ) {}

  label() {
    return m.cmd_set_field({ field: this.fieldName, value: String(this.after) })
  }

  async apply() {
    const result = await assetFieldApi.patch(this.assetId, [
      { field_id: this.fieldId, value: this.after },
    ])
    customFieldsStore.setFieldValues(this.assetId, userFields(result.fields))
  }

  async revert() {
    const result = await assetFieldApi.patch(this.assetId, [
      { field_id: this.fieldId, value: this.before },
    ])
    customFieldsStore.setFieldValues(this.assetId, userFields(result.fields))
  }
}
