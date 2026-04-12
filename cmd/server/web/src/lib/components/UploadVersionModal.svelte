<script lang="ts">
  import { versionApi, mimeCategory, type Asset } from '$lib/api'
  import Spinner from '$lib/components/ui/Spinner.svelte'
  import { Upload, X } from '@lucide/svelte'
  import Hint from './ui/Hint.svelte'
  import Feedback from './ui/Feedback.svelte'

  interface Props {
    asset: Asset
    onclose: () => void
    onuploaded: (updatedAsset: Asset) => void
  }

  let { asset, onclose, onuploaded }: Props = $props()

  let file = $state<File | null>(null)
  let comment = $state('')
  let uploading = $state(false)
  let progress = $state(0)
  let error = $state('')
  let mimeWarning = $state('')
  let dragging = $state(false)

  const assetCategory = $derived(mimeCategory(asset.mime_type))

  function checkMimeCategory(f: File) {
    const uploadCat = mimeCategory(f.type)
    if (uploadCat !== assetCategory) {
      mimeWarning = `Warning: you are replacing a ${assetCategory} with a ${uploadCat}. This is allowed but unusual.`
    } else if (f.type !== asset.mime_type) {
      mimeWarning = `Note: format will change from ${asset.mime_type} to ${f.type}.`
    } else {
      mimeWarning = ''
    }
  }

  function handleFileInput(e: Event) {
    const input = e.currentTarget as HTMLInputElement
    const f = input.files?.[0]
    if (f) { file = f; checkMimeCategory(f) }
  }

  function handleDrop(e: DragEvent) {
    e.preventDefault()
    dragging = false
    const f = e.dataTransfer?.files[0]
    if (f) { file = f; checkMimeCategory(f) }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') onclose()
  }

  async function handleSubmit() {
    if (!file) return
    uploading = true
    error = ''
    progress = 0
    try {
      const res = await versionApi.upload(asset.id, file, comment, (pct) => { progress = pct })
      onuploaded(res.asset)
    } catch (e_) {
      error = e_ instanceof Error ? e_.message : 'Upload failed'
    } finally {
      uploading = false
    }
  }
</script>

<svelte:window onkeydown={handleKeydown} />

<!-- Backdrop -->
<div
  class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm"
  role="button"
  tabindex="-1"
  onclick={(e) => { if (e.target === e.currentTarget) onclose() }}
  onkeydown={(e) => e.key === 'Enter' && onclose()}
  aria-label="Close modal"
>
  <div
    class="relative w-full max-w-md rounded-2xl bg-white shadow-2xl dark:bg-gray-900"
    role="dialog"
    aria-modal="true"
  >
    <!-- Header -->
    <div class="flex items-center justify-between border-b border-gray-100 px-6 py-4 dark:border-gray-800">
      <h2 class="text-base font-semibold text-gray-900 dark:text-gray-100">Upload new version</h2>
      <button
        type="button"
        class="rounded-lg p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-gray-800 dark:hover:text-gray-300"
        onclick={onclose}
      >
        <X class="h-4 w-4" />
      </button>
    </div>

    <!-- Body -->
    <div class="space-y-4 p-6">
      <!-- Drop zone -->
      <!-- svelte-ignore a11y_no_static_element_interactions -->
      <div
        class="relative flex flex-col items-center justify-center gap-3 rounded-xl border-2 border-dashed py-8 transition-colors
          {dragging ? 'border-indigo-400 bg-indigo-50 dark:border-indigo-500 dark:bg-indigo-900/20' : 'border-gray-200 dark:border-gray-700'}
          {file ? 'border-emerald-400 bg-emerald-50 dark:border-emerald-600 dark:bg-emerald-900/20' : ''}"
        ondragover={(e) => { e.preventDefault(); dragging = true }}
        ondragleave={() => { dragging = false }}
        ondrop={handleDrop}
      >
        {#if file}
          <div class="flex flex-col items-center gap-1 text-center">
            <span class="text-md font-medium text-gray-800 dark:text-gray-100">{file.name}</span>
            <span class="text-sm text-gray-400">{(file.size / 1024 / 1024).toFixed(2)} MB · {file.type}</span>
          </div>
        {:else}
          <Upload class="h-8 w-8 text-gray-300 dark:text-gray-600" />
          <Hint>Drop a file here or click to browse</Hint>
        {/if}
        <input
          type="file"
          class="absolute inset-0 cursor-pointer opacity-0"
          oninput={handleFileInput}
          disabled={uploading}
        />
      </div>

      {#if mimeWarning}
        <p class="rounded-lg bg-amber-50 px-3 py-2 text-sm text-amber-700 dark:bg-amber-900/30 dark:text-amber-400">
          {mimeWarning}
        </p>
      {/if}

      <!-- Comment -->
      <div>
        <label class="mb-1.5 block text-sm font-medium text-gray-500 dark:text-gray-400" for="version-comment">
          What changed? <span class="font-normal">(optional)</span>
        </label>
        <textarea
          id="version-comment"
          class="w-full rounded-xl border border-gray-200 bg-white px-3 py-2 text-md text-gray-800 placeholder-gray-400 focus:border-indigo-400 focus:outline-none dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100 dark:placeholder-gray-500"
          rows="2"
          maxlength="500"
          placeholder="e.g. Colour corrected, logo removed"
          bind:value={comment}
          disabled={uploading}
        ></textarea>
      </div>

      {#if uploading}
        <div class="h-1.5 w-full overflow-hidden rounded-full bg-gray-100 dark:bg-gray-700">
          <div
            class="h-full rounded-full bg-indigo-500 transition-all"
            style="width: {progress}%"
          ></div>
        </div>
      {/if}

      <Feedback {error} />
    </div>

    <!-- Footer -->
    <div class="flex justify-end gap-3 border-t border-gray-100 px-6 py-4 dark:border-gray-800">
      <button
        type="button"
        class="rounded-lg border border-gray-200 px-4 py-2 text-md text-gray-600 hover:bg-gray-50 dark:border-gray-700 dark:text-gray-300 dark:hover:bg-gray-800"
        onclick={onclose}
        disabled={uploading}
      >
        Cancel
      </button>
      <button
        type="button"
        class="flex items-center gap-2 rounded-lg bg-indigo-600 px-4 py-2 text-md font-medium text-white hover:bg-indigo-700 disabled:opacity-50"
        disabled={!file || uploading}
        onclick={handleSubmit}
      >
        {#if uploading}
          <Spinner size="sm" />
          Uploading… {progress}%
        {:else}
          <Upload class="h-4 w-4" />
          Upload version
        {/if}
      </button>
    </div>
  </div>
</div>
