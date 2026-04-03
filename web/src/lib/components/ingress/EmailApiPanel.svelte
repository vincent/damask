<script lang="ts">
  import { Copy, CheckCircle, Mail } from '@lucide/svelte'
  import type { IngressSource } from '$lib/api/models'
  import { toastStore } from '$lib/stores/toast.svelte'

  interface Props {
    source: IngressSource
  }

  let { source }: Props = $props()

  // The ingest address is stored in config.address by the email_api source
  const address = $derived((source.config as Record<string, string>).address ?? '')

  let copied = $state(false)

  async function copyAddress() {
    if (!address) return
    try {
      await navigator.clipboard.writeText(address)
      copied = true
      toastStore.show('Address copied!')
      setTimeout(() => { copied = false }, 2000)
    } catch {
      toastStore.show('Could not copy', 'error')
    }
  }

  function openMailto() {
    if (!address) return
    const subject = encodeURIComponent('Damask ingest test')
    const body = encodeURIComponent('Attach a file to this email to test your ingest source.')
    window.open(`mailto:${address}?subject=${subject}&body=${body}`)
  }
</script>

<div class="space-y-5">
  <div class="rounded-xl border border-indigo-100 bg-indigo-50/60 p-5 dark:border-indigo-900/40 dark:bg-indigo-900/20">
    <p class="mb-3 text-xs font-semibold uppercase tracking-widest text-indigo-500 dark:text-indigo-400">
      Your ingest address
    </p>

    {#if address}
      <div class="flex items-center gap-2 rounded-lg border border-indigo-200 bg-white px-3 py-2 dark:border-indigo-800 dark:bg-gray-900">
        <Mail class="h-4 w-4 shrink-0 text-indigo-400" />
        <span class="flex-1 font-mono text-sm text-gray-800 dark:text-gray-200 break-all">{address}</span>
        <button
          type="button"
          class="shrink-0 text-gray-400 transition-colors hover:text-indigo-600 dark:hover:text-indigo-400"
          onclick={copyAddress}
          title="Copy address"
        >
          {#if copied}
            <CheckCircle class="h-4 w-4 text-green-500" />
          {:else}
            <Copy class="h-4 w-4" />
          {/if}
        </button>
      </div>
    {:else}
      <p class="text-sm text-gray-400 dark:text-gray-500 italic">
        Address not yet assigned. Contact your workspace owner.
      </p>
    {/if}

    <p class="mt-3 text-xs text-gray-500 dark:text-gray-400">
      Send files as email attachments to this address. Supported formats: images, video, PDF, and common creative files. Max 25 MB per file.
    </p>
  </div>

  {#if address}
    <button
      type="button"
      class="flex w-full items-center justify-center gap-2 rounded-lg border border-gray-200 bg-white px-4 py-2.5 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200 dark:hover:bg-gray-700"
      onclick={openMailto}
    >
      <Mail class="h-4 w-4" />
      Send a test email
    </button>
  {/if}
</div>
