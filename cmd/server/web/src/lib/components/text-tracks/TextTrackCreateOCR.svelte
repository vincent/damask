<script lang="ts">
  import { m } from '$lib/paraglide/messages'

  interface Props {
    creating?: boolean
    onCreate: (params: { lang: string; output_format: string }) => Promise<void>
  }

  let { creating = false, onCreate }: Props = $props()

  let lang = $state('eng')
  let outputFormat = $state('txt')

  const languages = [
    ['eng', 'English'],
    ['fra', 'French'],
    ['spa', 'Spanish'],
    ['deu', 'German'],
    ['ita', 'Italian'],
    ['cat', 'Catalan'],
  ]
</script>

<div class="space-y-4">
  <p class="text-sm text-[var(--text-muted)]">
    {m.text_tracks_ocr_description()}
  </p>

  <label class="block space-y-2">
    <span class="text-sm font-medium text-[var(--text-primary)]">
      {m.text_tracks_ocr_lang_label()}
    </span>
    <select bind:value={lang} class="w-full rounded-xl border border-gray-200 bg-white px-3 py-2 text-sm dark:border-gray-700 dark:bg-gray-900">
      {#each languages as [value, label]}
        <option {value}>{label}</option>
      {/each}
    </select>
  </label>

  <label class="block space-y-2">
    <span class="text-sm font-medium text-[var(--text-primary)]">
      {m.text_tracks_ocr_format_label()}
    </span>
    <select bind:value={outputFormat} class="w-full rounded-xl border border-gray-200 bg-white px-3 py-2 text-sm dark:border-gray-700 dark:bg-gray-900">
      <option value="txt">{m.text_tracks_ocr_format_txt()}</option>
      <option value="hocr">{m.text_tracks_ocr_format_hocr()}</option>
    </select>
  </label>

  {#if outputFormat === 'hocr'}
    <p class="text-xs text-[var(--text-muted)]">
      {m.text_tracks_ocr_format_hocr_hint()}
    </p>
  {/if}

  <button
    type="button"
    class="w-full rounded-xl bg-indigo-600 px-4 py-2 text-sm font-medium text-white transition hover:bg-indigo-500 disabled:cursor-not-allowed disabled:opacity-60"
    disabled={creating}
    onclick={() => onCreate({ lang, output_format: outputFormat })}
  >
    {creating ? m.queuing_() : m.text_tracks_ocr_submit()}
  </button>
</div>
