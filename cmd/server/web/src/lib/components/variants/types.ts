import type { Component } from 'svelte'
import type { VariantTab } from './VariantsTool.svelte'

export interface VariantToolDef {
  key: VariantTab
  label: string
  sublabel: string
  icon: Component
  showFor: (mimeType: string) => boolean
}
