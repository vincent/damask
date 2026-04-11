<script lang="ts">
  import { Mail, Server, HardDrive, Cloud, Inbox, ChevronLeft, CheckCircle, AlertCircle } from '@lucide/svelte'
  import type { IngressSource, IngressSourceType } from '$lib/api/models'
  import { ingressStore } from '$lib/stores/ingress.svelte'
  import { projectsStore } from '$lib/stores/projects.svelte'
  import { ingressApi } from '$lib/api'
  import Modal from '$lib/components/ui/Modal.svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import Input from '$lib/components/ui/Input.svelte'
  import SourceConfigForm from './SourceConfigForm.svelte'
  import Hint from '../ui/Hint.svelte'

  interface Props {
    open?: boolean
    onadded: (source: IngressSource) => void
    onclose: () => void
  }

  let { open = $bindable(false), onadded, onclose }: Props = $props()

  type Step = 'pick' | 'configure'

  const SOURCE_TYPES: { type: IngressSourceType; label: string; desc: string; icon: typeof Mail }[] = [
    { type: 'email_api', label: 'Own email address', desc: 'Zero config — get a unique ingest address', icon: Mail },
    { type: 'imap', label: 'IMAP mailbox', desc: 'Pull attachments from any IMAP mailbox', icon: Inbox },
    { type: 'sftp', label: 'SFTP server', desc: 'Watch a remote directory for new files', icon: Server },
    { type: 'dav', label: 'WebDAV / Nextcloud', desc: 'Watch a WebDAV collection', icon: HardDrive },
    { type: 's3', label: 'S3-compatible bucket', desc: 'AWS S3, R2, MinIO, Backblaze B2…', icon: Cloud },
  ]

  const POLL_INTERVALS = [
    { label: '5 minutes', value: 5 },
    { label: '15 minutes', value: 15 },
    { label: '30 minutes', value: 30 },
    { label: '1 hour', value: 60 },
    { label: '6 hours', value: 360 },
  ]

  let step = $state<Step>('pick')
  let selectedType = $state<IngressSourceType | null>(null)

  // Common fields
  let label = $state('')
  let destProjectId = $state('')
  let pollIntervalMin = $state(15)

  // Source-specific config (managed by SourceConfigForm)
  let sourceConfig = $state<Record<string, unknown>>({})

  // Test connection state
  let testStatus = $state<'idle' | 'testing' | 'ok' | 'error'>('idle')
  let testError = $state('')
  let createdSourceId = $state<string | null>(null)

  // Submission
  let saving = $state(false)
  let errors = $state<Record<string, string>>({})

  function pickType(type: IngressSourceType) {
    selectedType = type
    step = 'configure'
    label = ''
    destProjectId = projectsStore.projects[0]?.id ?? ''
    pollIntervalMin = 15
    sourceConfig = {}
    testStatus = 'idle'
    testError = ''
    createdSourceId = null
    errors = {}
  }

  function back() {
    step = 'pick'
    selectedType = null
    createdSourceId = null
  }

  function validate(): boolean {
    const e: Record<string, string> = {}
    if (!label.trim()) e.label = 'Label is required'
    if (!destProjectId) e.destProjectId = 'Destination project is required'
    errors = e
    return Object.keys(e).length === 0
  }

  // Create (or find already-created) source to enable "Test connection"
  async function ensureSourceCreated(): Promise<string | null> {
    if (createdSourceId) return createdSourceId
    if (!validate()) return null

    saving = true
    try {
      const src = await ingressStore.createSource({
        type: selectedType!,
        label: label.trim(),
        config: sourceConfig,
        dest_project_id: destProjectId || null,
        poll_interval_min: pollIntervalMin,
        enabled: false, // not enabled until user finishes
      })
      if (!src) return null
      createdSourceId = src.id
      return src.id
    } finally {
      saving = false
    }
  }

  async function testConnection() {
    const id = await ensureSourceCreated()
    if (!id) return
    testStatus = 'testing'
    testError = ''
    try {
      await ingressApi.test(id)
      testStatus = 'ok'
    } catch (e: unknown) {
      testStatus = 'error'
      testError = e instanceof Error ? e.message : 'Connection test failed'
    }
  }

  async function handleSave() {
    if (!validate()) return
    saving = true
    try {
      let src: IngressSource | null
      if (createdSourceId) {
        // Update the draft with final values + enable it
        src = await ingressStore.updateSource(createdSourceId, {
          label: label.trim(),
          config: sourceConfig,
          dest_project_id: destProjectId || null,
          poll_interval_min: pollIntervalMin,
          enabled: true,
        })
      } else {
        src = await ingressStore.createSource({
          type: selectedType!,
          label: label.trim(),
          config: sourceConfig,
          dest_project_id: destProjectId || null,
          poll_interval_min: pollIntervalMin,
          enabled: true,
        })
      }
      if (src) {
        open = false
        onadded(src)
      }
    } finally {
      saving = false
    }
  }

  const selectedTypeMeta = $derived(SOURCE_TYPES.find((t) => t.type === selectedType))
</script>

<Modal bind:open {onclose}>
  {#if step === 'pick'}
    <!-- Step 1: Type picker -->
    <div class="px-6 py-5">
      <h2 class="text-xl font-semibold text-gray-900 dark:text-gray-50">Add ingress source</h2>
      <Hint>Choose where files will come from.</Hint>
    </div>

    <div class="grid grid-cols-1 gap-3 px-6 pb-6 sm:grid-cols-2">
      {#each SOURCE_TYPES as { type, label: typeLabel, desc, icon: Icon }}
        <button
          type="button"
          class="flex items-start gap-3 rounded-xl border border-gray-100 bg-white p-4 text-left transition-colors
            hover:border-indigo-300 hover:bg-indigo-50/40
            dark:border-gray-800 dark:bg-gray-900 dark:hover:border-indigo-700 dark:hover:bg-indigo-900/20"
          onclick={() => pickType(type)}
        >
          <div class="mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-indigo-50 text-indigo-500 dark:bg-indigo-900/30 dark:text-indigo-400">
            <Icon class="h-5 w-5" />
          </div>
          <div>
            <Hint class="text-md font-semibold">{typeLabel}</Hint>
            <Hint class="text-sm">{desc}</Hint>
          </div>
        </button>
      {/each}
    </div>

  {:else}
    <!-- Step 2: Configure -->
    <div class="flex items-center gap-3 border-b border-gray-100 px-6 py-4 dark:border-gray-800">
      <button
        type="button"
        class="flex h-8 w-8 items-center justify-center rounded-lg text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-800"
        onclick={back}
      >
        <ChevronLeft class="h-4 w-4" />
      </button>
      <div>
        <h2 class="text-base font-semibold text-gray-900 dark:text-gray-50">
          {selectedTypeMeta?.label}
        </h2>
        <Hint class="text-sm">{selectedTypeMeta?.desc}</Hint>
      </div>
    </div>

    <div class="space-y-5 px-6 py-5">
      <!-- Common fields -->
      <Input
        id="source-label"
        label="Label"
        placeholder="e.g. Client uploads inbox"
        bind:value={label}
        error={errors.label}
        required
      />

      <div class="grid grid-cols-2 gap-4">
        <!-- Destination project -->
        <div>
          <label for="dest-project" class="mb-1 block text-md font-medium text-gray-700 dark:text-gray-300">
            Destination project
          </label>
          <select
            id="dest-project"
            bind:value={destProjectId}
            class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-md text-gray-900 shadow-sm focus:border-indigo-400 focus:outline-none focus:ring-2 focus:ring-indigo-200 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100 dark:focus:border-indigo-500 dark:focus:ring-indigo-900"
          >
            <option value="">— none —</option>
            {#each projectsStore.projects as p (p.id)}
              <option value={p.id}>{p.name}</option>
            {/each}
          </select>
          {#if errors.destProjectId}
            <p class="mt-1 text-sm text-red-600 dark:text-red-400">{errors.destProjectId}</p>
          {/if}
        </div>

        <!-- Poll interval (hidden for email_api) -->
        {#if selectedType !== 'email_api'}
          <div>
            <label for="poll-interval" class="mb-1 block text-md font-medium text-gray-700 dark:text-gray-300">
              Poll interval
            </label>
            <select
              id="poll-interval"
              bind:value={pollIntervalMin}
              class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-md text-gray-900 shadow-sm focus:border-indigo-400 focus:outline-none focus:ring-2 focus:ring-indigo-200 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100 dark:focus:border-indigo-500 dark:focus:ring-indigo-900"
            >
              {#each POLL_INTERVALS as opt (opt.value)}
                <option value={opt.value}>{opt.label}</option>
              {/each}
            </select>
          </div>
        {/if}
      </div>

      <!-- Source-specific config form -->
      <SourceConfigForm type={selectedType!} bind:config={sourceConfig} />

      <!-- Test connection -->
      {#if selectedType !== 'email_api'}
        <div class="rounded-lg border border-gray-100 bg-gray-50 p-4 dark:border-gray-800 dark:bg-gray-800/50">
          <div class="flex items-center justify-between gap-3">
            <div class="text-md text-gray-600 dark:text-gray-300">
              {#if testStatus === 'idle'}
                Test the connection before saving.
              {:else if testStatus === 'testing'}
                Testing connection…
              {:else if testStatus === 'ok'}
                <span class="flex items-center gap-1 text-green-600 dark:text-green-400">
                  <CheckCircle class="h-4 w-4" /> Connection successful
                </span>
              {:else}
                <span class="flex items-center gap-1 text-red-600 dark:text-red-400">
                  <AlertCircle class="h-4 w-4" /> {testError}
                </span>
              {/if}
            </div>
            <Button
              variant="secondary"
              size="sm"
              loading={testStatus === 'testing'}
              onclick={testConnection}
            >
              Test connection
            </Button>
          </div>
        </div>
      {/if}
    </div>

    <!-- Footer actions -->
    <div class="flex items-center justify-end gap-2 border-t border-gray-100 px-6 py-4 dark:border-gray-800">
      <Button variant="secondary" onclick={onclose}>Cancel</Button>
      <Button
        variant="primary"
        loading={saving}
        onclick={handleSave}
      >
        Save source
      </Button>
    </div>
  {/if}
</Modal>
