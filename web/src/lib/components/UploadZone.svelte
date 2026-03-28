<script lang="ts">
  import { assetApi, type Asset } from '$lib/api/client'
  import { uploadsStore } from '$lib/stores/uploads.svelte'

  interface Props {
    onuploaded: (asset: Asset) => void
  }

  let { onuploaded }: Props = $props()

  let isDragging = $state(false)
  let fileInput: HTMLInputElement

  function handleDragOver(e: DragEvent) {
    e.preventDefault()
    if (e.dataTransfer) {
      e.dataTransfer.dropEffect = "copy";
    }
    isDragging = true
  }

  function handleDragLeave(e: DragEvent) {
    if (!(e.currentTarget as HTMLElement).contains(e.relatedTarget as Node)) {
      isDragging = false
    }
  }

  function handleDrop(e: DragEvent) {
    isDragging = false
    const files = Array.from(e.dataTransfer?.files ?? [])
    uploadFiles(files)
    e.preventDefault()
  }

  function handleFileInput(e: Event) {
    const files = Array.from((e.target as HTMLInputElement).files ?? [])
    uploadFiles(files)
    fileInput.value = ''
  }

  function uploadFiles(files: File[]) {
    for (const file of files) {
      const id = crypto.randomUUID()
      uploadsStore.add({ id, file, progress: 0, status: 'uploading' })

      assetApi
        .upload(file, (pct) => uploadsStore.update(id, { progress: pct }))
        .then((asset) => {
          uploadsStore.update(id, { status: 'processing', asset, progress: 100 })
          onuploaded(asset)
          pollUntilReady(id, asset.id)
        })
        .catch((err: Error) => {
          uploadsStore.update(id, { status: 'error', error: err.message })
        })
    }
  }

  function pollUntilReady(uploadId: string, assetId: string) {
    const interval = setInterval(async () => {
      try {
        const asset = await assetApi.get(assetId)
        if (asset.thumbnail_key.Valid) {
          uploadsStore.update(uploadId, { status: 'done', asset })
          clearInterval(interval)
        }
      } catch {
        clearInterval(interval)
      }
    }, 2000)
  }
</script>

<button
  type="button"
  aria-label="Upload zone — drag and drop files or click to browse"
  class="relative flex h-36 w-full cursor-pointer flex-col items-center justify-center gap-2 rounded-xl border-2 border-dashed transition-colors {isDragging
    ? 'border-blue-400 bg-blue-50'
    : 'border-gray-300 bg-gray-50 hover:border-gray-400 hover:bg-gray-100'}"
  ondragover={handleDragOver}
  ondragleave={handleDragLeave}
  ondrop={handleDrop}
  onclick={() => fileInput.click()}
>
  <svg
    class="h-8 w-8 {isDragging ? 'text-blue-400' : 'text-gray-400'}"
    fill="none"
    viewBox="0 0 24 24"
    stroke="currentColor"
  >
    <path
      stroke-linecap="round"
      stroke-linejoin="round"
      stroke-width="1.5"
      d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12"
    />
  </svg>
  <p class="text-sm {isDragging ? 'text-blue-600' : 'text-gray-500'}">
    {isDragging ? 'Drop files here' : 'Drag & drop files, or click to browse'}
  </p>

  <input
    bind:this={fileInput}
    type="file"
    multiple
    class="hidden"
    onchange={handleFileInput}
  />
</button>

<!-- Upload queue -->
{#if uploadsStore.items.length > 0}
  <div class="mt-3 flex flex-col gap-2">
    {#each uploadsStore.items as item (item.id)}
      <div class="flex items-center gap-3 rounded-lg border border-gray-200 bg-white px-3 py-2">
        <div class="min-w-0 flex-1">
          <p class="truncate text-sm font-medium text-gray-800">{item.file.name}</p>
          {#if item.status === 'uploading'}
            <div class="mt-1 h-1.5 overflow-hidden rounded-full bg-gray-200">
              <div
                class="h-full rounded-full bg-blue-500 transition-all"
                style="width: {item.progress}%"
              ></div>
            </div>
          {:else if item.status === 'processing'}
            <p class="text-xs text-amber-600">Processing thumbnail…</p>
          {:else if item.status === 'done'}
            <p class="text-xs text-green-600">Done</p>
          {:else if item.status === 'error'}
            <p class="truncate text-xs text-red-600">{item.error}</p>
          {/if}
        </div>

        {#if item.status === 'done' || item.status === 'error'}
          <button
            type="button"
            class="shrink-0 text-gray-400 hover:text-gray-600"
            onclick={() => uploadsStore.remove(item.id)}
            aria-label="Dismiss"
          >
            <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        {/if}
      </div>
    {/each}
  </div>
{/if}
