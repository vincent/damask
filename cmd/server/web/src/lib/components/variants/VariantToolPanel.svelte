<script lang="ts">
  import type { Asset } from '$lib/api'
  import { m } from '$lib/paraglide/messages'
  import { X } from '@lucide/svelte'
  import { fly } from 'svelte/transition'
  import { ALL_VARIANT_TOOLS } from './toolDefs'
  import VariantsTool from './VariantsTool.svelte'
  import type { VariantTab } from './VariantsTool.svelte'

  interface Props {
    tool: VariantTab
    asset: Asset
    creating: boolean
    handleCreate: (type: string, params: object) => void
    onClose: () => void
  }

  let { tool, asset, creating, handleCreate, onClose }: Props = $props()

  const messages = m as unknown as Record<string, (() => string) | undefined>
  const msg = (key: string) => messages[key]?.() ?? key
  const toolDef = $derived(ALL_VARIANT_TOOLS.find((item) => item.key === tool))
</script>

<div
  class="variant-tool-panel pointer-events-auto"
  data-testid="variant-tool-panel"
  role="presentation"
  transition:fly={{ x: -16, duration: 200, opacity: 0 }}
  onclick={(event) => event.stopPropagation()}
  onkeydown={(event) => event.stopPropagation()}
>
  <header class="panel-header">
    <div class="panel-title-group">
      <h2>{toolDef ? msg(toolDef.label) : tool}</h2>
      {#if toolDef}
        <p>{msg(toolDef.sublabel)}</p>
      {/if}
    </div>
    <button
      type="button"
      class="close-button"
      disabled={creating}
      onclick={() => {
        if (!creating) onClose()
      }}
      aria-label={msg('variant_tool_panel_close')}
      title={msg('variant_tool_panel_close')}
    >
      <X size={18} strokeWidth={1.75} />
    </button>
  </header>

  <div class="panel-body">
    <VariantsTool {tool} {asset} {creating} {handleCreate} />
  </div>
</div>

<style>
  .variant-tool-panel {
    --tool-panel-width: 350px;

    position: absolute;
    top: 0;
    bottom: 0;
    left: 60px;
    z-index: 20;
    display: flex;
    width: var(--tool-panel-width);
    flex-direction: column;
    border-right: 1px solid var(--border-subtle, rgb(229 231 235 / 0.72));
    background: var(--surface-panel, rgb(255 255 255 / 0.96));
    box-shadow: 16px 0 36px rgb(15 23 42 / 0.12);
    overflow: auto;
    backdrop-filter: blur(18px);
  }

  :global(.dark) .variant-tool-panel {
    border-right-color: var(--dark-border-subtle, rgb(55 65 81 / 0.72));
    background: var(--dark-surface-panel, rgb(17 24 39 / 0.96));
    box-shadow: 18px 0 42px rgb(0 0 0 / 0.32);
  }

  .panel-header {
    display: flex;
    flex-shrink: 0;
    align-items: flex-start;
    gap: 12px;
    border-bottom: 1px solid var(--border-subtle, rgb(229 231 235 / 0.72));
    padding: 16px 14px 14px 18px;
  }

  :global(.dark) .panel-header {
    border-bottom-color: var(--dark-border-subtle, rgb(55 65 81 / 0.72));
  }

  .panel-title-group {
    min-width: 0;
    flex: 1;
  }

  .panel-title-group h2 {
    margin: 0;
    color: var(--text-primary, rgb(17 24 39));
    font-size: 0.95rem;
    font-weight: 700;
    line-height: 1.25;
  }

  :global(.dark) .panel-title-group h2 {
    color: var(--dark-text-primary, white);
  }

  .panel-title-group p {
    margin: 4px 0 0;
    color: var(--text-muted, rgb(107 114 128));
    font-size: 12px;
    line-height: 1.35;
  }

  :global(.dark) .panel-title-group p {
    color: var(--dark-text-muted, rgb(156 163 175));
  }

  .close-button {
    display: flex;
    width: 32px;
    height: 32px;
    flex-shrink: 0;
    cursor: pointer;
    align-items: center;
    justify-content: center;
    border: 0;
    border-radius: 8px;
    background: transparent;
    color: var(--text-secondary, rgb(75 85 99));
    transition:
      background 120ms ease,
      color 120ms ease,
      opacity 120ms ease;
  }

  .close-button:not(:disabled):hover {
    background: var(--surface-hover, rgb(249 250 251));
    color: var(--text-primary, rgb(17 24 39));
  }

  .close-button:disabled {
    cursor: not-allowed;
    opacity: 0.45;
  }

  :global(.dark) .close-button {
    color: var(--dark-text-secondary, rgb(209 213 219));
  }

  :global(.dark) .close-button:not(:disabled):hover {
    background: var(--dark-surface-hover, rgb(31 41 55));
    color: var(--dark-text-primary, white);
  }

  .panel-body {
    min-height: 0;
    flex: 1;
    padding: 16px 18px 20px;
  }
</style>
