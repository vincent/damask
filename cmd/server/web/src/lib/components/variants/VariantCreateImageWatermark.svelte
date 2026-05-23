<script lang="ts">
  import { authStore } from '$lib/stores/auth.svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import { variantApi, type Asset, type WatermarkAsset } from '$lib/api'
  import { generateDraft } from '$lib/api/drafts'
  import { toastStore } from '$lib/stores/toast.svelte'
  import { m } from '$lib/paraglide/messages'
  import VariantDraftSession from './VariantDraftSession.svelte'

  interface Props {
    asset: Asset
    creating: boolean
    handleCreate: (kind: string, params: Record<string, unknown>) => void
    onDone?: () => void
    onDraftStarted?: (nonce: string) => void
    sessionActive?: boolean
  }

  let {
    asset,
    creating,
    handleCreate: _handleCreate,
    onDone,
    onDraftStarted,
    sessionActive = false,
  }: Props = $props()

  type Phase = 'form' | 'drafting'

  const kind = 'image_watermark'
  let phase = $state<Phase>('form')
  let opacity = $state(50)
  let watermarkAsset = $state<WatermarkAsset | null>(null)
  let resolveError = $state('')
  let resolving = $state(false)
  let resolveToken = 0
  let submitting = $state(false)
  let sessionRef = $state<ReturnType<typeof VariantDraftSession> | undefined>(
    undefined
  )

  $effect(() => {
    const assetId = asset.id
    resolveToken += 1
    const token = resolveToken
    watermarkAsset = null
    resolveError = ''
    resolving = true

    void variantApi
      .resolveWatermark(assetId)
      .then((result) => {
        if (token !== resolveToken) return
        watermarkAsset = result
      })
      .catch((error: unknown) => {
        if (token !== resolveToken) return
        resolveError =
          error instanceof Error ? error.message : m.watermark_missing()
      })
      .finally(() => {
        if (token === resolveToken) resolving = false
      })
  })

  $effect(() => {
    if (!sessionActive) phase = 'form'
  })

  async function handlePreview() {
    submitting = true
    try {
      const res = await generateDraft(asset.id, kind, {
        opacity: opacity / 100,
        watermark_asset_id: watermarkAsset?.id ?? '',
      })
      if (onDraftStarted) {
        onDraftStarted(res.draft_key)
      } else {
        phase = 'drafting'
        await Promise.resolve()
        sessionRef?.addDraft(res.draft_key)
      }
    } catch (e: unknown) {
      toastStore.show(
        e instanceof Error ? e.message : m.variant_create_failed(),
        'error'
      )
    } finally {
      submitting = false
    }
  }

  function handleAddMore() {
    phase = 'form'
  }

  function handleDone() {
    onDone?.()
  }
</script>

{#if phase === 'form' || onDraftStarted}
  <div class="space-y-5">
    {#if resolving}
      <div class="notice">{m.loading()}</div>
    {:else if resolveError || !watermarkAsset}
      <div class="notice notice--error">
        {resolveError || m.watermark_missing()}
      </div>
    {:else}
      <div class="notice">
        <p class="notice-title">{watermarkAsset.name}</p>
        <p class="notice-body">
          {m.watermark_scope_label()}: {watermarkAsset.scope}
        </p>
      </div>
    {/if}

    <div>
      <label for="variant-{kind}-opacity" class="field-label">
        {m.opacity()}: <span class="quality-val">{opacity}%</span>
      </label>
      <input
        id="variant-{kind}-opacity"
        type="range"
        min="1"
        max="100"
        bind:value={opacity}
        class="range-input"
      />
    </div>

    <Button
      disabled={submitting || authStore.role === 'viewer' || !watermarkAsset}
      onclick={handlePreview}
      class="w-full"
    >
      {submitting ? m.queuing_() : m.variants_draft_preview_button()}
    </Button>
  </div>
{:else}
  <VariantDraftSession
    bind:this={sessionRef}
    assetId={asset.id}
    onDone={handleDone}
    onAddMore={handleAddMore}
  />
{/if}

<style>
  .notice {
    border-radius: 8px;
    border: 1px solid var(--border);
    background: var(--bg-surface);
    padding: 12px 14px;
  }
  .notice--error {
    border-color: oklch(75% 0.12 25 / 0.5);
    background: oklch(97% 0.03 25);
    color: var(--danger, #b42318);
    font-size: 0.875rem;
  }
  :global(.dark) .notice--error {
    background: oklch(25% 0.05 25 / 0.2);
    color: oklch(75% 0.12 25);
  }
  .notice-title {
    font-size: 0.875rem;
    font-weight: 600;
    color: var(--text-primary);
    margin-bottom: 4px;
  }
  .notice-body {
    font-size: 0.8125rem;
    color: var(--text-muted);
    text-transform: capitalize;
  }
  .field-label {
    display: block;
    margin-bottom: 4px;
    font-size: 0.75rem;
    font-weight: 500;
    color: var(--text-secondary);
  }
  .range-input {
    width: 100%;
    accent-color: var(--accent-cta);
  }
  .quality-val {
    font-weight: 600;
    color: var(--accent-cta);
  }
</style>
