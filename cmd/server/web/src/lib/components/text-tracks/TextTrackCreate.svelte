<script lang="ts">
  import { textTrackApi, type Asset } from '$lib/api'
  import { m } from '$lib/paraglide/messages'
  import TextTrackCreateOCR from './TextTrackCreateOCR.svelte'

  interface Props {
    asset: Asset
    onCreated: () => Promise<void> | void
  }

  let { asset, onCreated }: Props = $props()

  let selectedSource = $state<'ocr' | 'manual'>(
    asset.mime_type.startsWith('image/') ? 'ocr' : 'manual'
  )
  let creating = $state(false)
  let error = $state('')
  let manualContent = $state('')

  async function createOCR(params: { lang: string; output_format: string }) {
    creating = true
    error = ''
    try {
      await textTrackApi.create(asset.id, {
        source: 'ocr',
        lang: params.lang,
        params,
      })
      await onCreated()
    } catch (e) {
      error = e instanceof Error ? e.message : m.text_tracks_error_generic()
    } finally {
      creating = false
    }
  }

  async function createManual() {
    creating = true
    error = ''
    try {
      await textTrackApi.create(asset.id, {
        source: 'manual',
        params: { content: manualContent },
      })
      manualContent = ''
      await onCreated()
    } catch (e) {
      error = e instanceof Error ? e.message : m.text_tracks_error_generic()
    } finally {
      creating = false
    }
  }
</script>

<div class="space-y-4 rounded-2xl border border-gray-100 bg-gray-50/80 p-4 dark:border-gray-800 dark:bg-gray-900/60">
  {#if asset.mime_type.startsWith('image/')}
    <label class="block space-y-2">
      <span class="text-sm font-medium text-[var(--text-primary)]">
        {m.text_tracks_source_label()}
      </span>
      <select bind:value={selectedSource} class="w-full rounded-xl border border-gray-200 bg-white px-3 py-2 text-sm dark:border-gray-700 dark:bg-gray-950">
        <option value="ocr">{m.text_tracks_source_ocr()}</option>
        <option value="manual">{m.text_tracks_source_manual()}</option>
      </select>
    </label>
  {/if}

  {#if selectedSource === 'ocr'}
    <TextTrackCreateOCR {creating} onCreate={createOCR} />
  {:else}
    <label class="block space-y-2">
      <span class="text-sm font-medium text-[var(--text-primary)]">
        {m.text_tracks_manual_content_label()}
      </span>
      <textarea
        bind:value={manualContent}
        rows="5"
        class="w-full rounded-xl border border-gray-200 bg-white px-3 py-2 text-sm dark:border-gray-700 dark:bg-gray-950"
        placeholder={m.text_tracks_manual_content_placeholder()}
      ></textarea>
    </label>
    <button
      type="button"
      class="w-full rounded-xl bg-[var(--text-primary)] px-4 py-2 text-sm font-medium text-white transition hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-60"
      disabled={creating}
      onclick={createManual}
    >
      {creating ? m.queuing_() : m.text_tracks_manual_submit()}
    </button>
  {/if}

  {#if error}
    <p class="text-sm text-red-600 dark:text-red-400">{error}</p>
  {/if}
</div>
