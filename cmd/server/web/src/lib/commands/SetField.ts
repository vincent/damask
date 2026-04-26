import type { Command } from './types'
import { assetFieldApi } from '$lib/api'
import { customFieldsStore } from '$lib/stores/customFields.svelte'
import { m } from '$lib/paraglide/messages'

export class SetAssetField implements Command {
  constructor(
    private assetId: string,
    private fieldId: string,
    private fieldName: string,
    private before: string | number | boolean | null,
    private after: string | number | boolean,
  ) {}

  label() { return m.cmd_set_field({ field: this.fieldName, value: String(this.after) }) }

  async apply() {
    const result = await assetFieldApi.patch(this.assetId, [{ field_id: this.fieldId, value: this.after }])
    customFieldsStore.setFieldValues(this.assetId, result.fields)
  }

  async revert() {
    const result = await assetFieldApi.patch(this.assetId, [{ field_id: this.fieldId, value: this.before }])
    customFieldsStore.setFieldValues(this.assetId, result.fields)
  }
}
