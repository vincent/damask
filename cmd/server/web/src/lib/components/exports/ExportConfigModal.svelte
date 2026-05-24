<script lang="ts">
  import { CheckCircle, AlertCircle } from '@lucide/svelte'
  import { projectsStore } from '$lib/stores/projects.svelte'
  import { exportsApi, type ExportConfig } from '$lib/api/exports'
  import Modal from '$lib/components/ui/Modal.svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import Hint from '$lib/components/ui/Hint.svelte'
  import Feedback from '$lib/components/ui/Feedback.svelte'
  import DestinationTypePicker from '$lib/components/destinations/DestinationTypePicker.svelte'
  import SftpConfigForm from '$lib/components/destinations/SftpConfigForm.svelte'
  import GDriveConfigForm from '$lib/components/destinations/GDriveConfigForm.svelte'

  interface Props {
    open?: boolean
    config: ExportConfig | null
    onSave: (config: ExportConfig) => void
    onClose: () => void
  }

  let {
    open = $bindable(false),
    config = null,
    onSave,
    onClose,
  }: Props = $props()

  const isEdit = $derived(config !== null)
  // svelte-ignore state_referenced_locally
  let cfg = $state(config || ({} as ExportConfig))

  // Step 1 state
  let projectId = $state(cfg.project_id ?? '')
  let versions = $state<'current' | 'all'>(cfg.versions ?? 'current')
  let includeVariants = $state(cfg.include_variants ?? false)

  // Step 2 state
  let destType = $state<'sftp' | 'gdrive'>(cfg.dest_type ?? 'sftp')
  let sftpConfig = $state({
    host: '',
    port: 22,
    username: '',
    password: '',
    private_key: '',
    remote_path: '/',
  })
  let gdriveConfig = $state({
    connection_id: '',
    workspace_id: '',
    folder_id: '',
    folder_name: '',
  })
  let scheduleType = $state<'manual' | 'after_quiet'>(
    cfg.schedule_type ?? 'manual'
  )
  let quietValue = $state(
    cfg.quiet_minutes
      ? cfg.quiet_minutes >= 60
        ? cfg.quiet_minutes / 60
        : cfg.quiet_minutes
      : 1
  )
  let quietUnit = $state<'min' | 'h'>(
    cfg.quiet_minutes && cfg.quiet_minutes >= 60 ? 'h' : 'min'
  )
  let label = $state(cfg.label ?? '')

  // Validation state
  let validating = $state(false)
  let validateResult = $state<'ok' | 'error' | null>(null)
  let validateError = $state('')
  let fieldErrors = $state<Record<string, string>>({})

  let step = $state(1)
  let saving = $state(false)
  let saveError = $state('')

  const quietMinutes = $derived(
    scheduleType === 'after_quiet'
      ? quietUnit === 'h'
        ? quietValue * 60
        : quietValue
      : undefined
  )

  function step1Valid() {
    return projectId !== ''
  }

  function buildPayload() {
    const destConfig = destType === 'sftp' ? sftpConfig : gdriveConfig
    return {
      project_id: projectId,
      label,
      dest_type: destType,
      dest_config: destConfig,
      versions,
      include_variants: includeVariants,
      schedule_type: scheduleType,
      quiet_minutes: quietMinutes,
    }
  }

  async function validate() {
    validating = true
    validateResult = null
    validateError = ''
    try {
      const payload = buildPayload()
      await exportsApi.validateDest(payload)
      validateResult = 'ok'
    } catch (e: unknown) {
      validateResult = 'error'
      validateError = e instanceof Error ? e.message : 'Connection failed'
    } finally {
      validating = false
    }
  }

  async function save() {
    saving = true
    saveError = ''
    fieldErrors = {}
    try {
      const payload = buildPayload()
      let saved: ExportConfig
      if (isEdit && config) {
        saved = await exportsApi.update(cfg.id, payload)
      } else {
        saved = await exportsApi.create(payload)
      }
      onSave(saved)
    } catch (e: unknown) {
      if (e && typeof e === 'object' && 'fields' in e) {
        fieldErrors = (e as { fields: Record<string, string> }).fields
      }
      saveError = e instanceof Error ? e.message : 'Failed to save'
    } finally {
      saving = false
    }
  }
</script>

<Modal
  bind:open
  onclose={() => {
    step = 1
    onClose()
  }}
>
  <!-- Header -->
  <div class="border-b border-gray-100 px-6 py-4 dark:border-gray-800">
    <h2 class="text-base font-semibold text-gray-900 dark:text-gray-50">
      {isEdit ? 'Edit export' : 'New export'}
    </h2>
    <Hint class="text-sm">
      Step {step} of 2 — {step === 1 ? 'What to export' : 'Where & when'}
    </Hint>
  </div>

  <!-- Body -->
  <div class="space-y-5 px-6 py-5">
    {#if step === 1}
      <!-- Project -->
      <div>
        <label
          for="export-project"
          class="text-md mb-1 block font-medium text-gray-700 dark:text-gray-300"
        >
          Project
        </label>
        <select
          id="export-project"
          class="text-md w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-gray-900 shadow-sm focus:border-indigo-400 focus:ring-2 focus:ring-indigo-200 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100 dark:focus:border-indigo-500 dark:focus:ring-indigo-900"
          bind:value={projectId}
        >
          <option value="">Select a project</option>
          {#each projectsStore.projects as p}
            <option value={p.id}>{p.name}</option>
          {/each}
        </select>
        {#if fieldErrors.project_id}
          <p class="mt-1 text-xs text-red-500">{fieldErrors.project_id}</p>
        {/if}
      </div>

      <!-- Versions -->
      <div>
        <p class="text-md mb-2 font-medium text-gray-700 dark:text-gray-300">
          Versions
        </p>
        <div class="space-y-2">
          {#each [{ value: 'current', label: 'Current version only', desc: 'Export only the latest file for each asset' }, { value: 'all', label: 'All versions', desc: 'Include all historical versions in versioned subfolders' }] as opt}
            <label
              class="flex cursor-pointer items-start gap-3 rounded-lg border p-3 transition-colors {versions ===
              opt.value
                ? 'border-indigo-300 bg-indigo-50/40 dark:border-indigo-700 dark:bg-indigo-900/20'
                : 'border-gray-100 hover:border-gray-200 dark:border-gray-800 dark:hover:border-gray-700'}"
            >
              <input
                type="radio"
                bind:group={versions}
                value={opt.value}
                class="mt-0.5 accent-indigo-600"
              />
              <div>
                <p class="text-sm font-medium text-gray-900 dark:text-gray-100">
                  {opt.label}
                </p>
                <Hint class="text-xs">{opt.desc}</Hint>
              </div>
            </label>
          {/each}
        </div>
      </div>

      <!-- Include variants -->
      <label
        class="flex cursor-pointer items-start gap-3 rounded-lg border p-3 transition-colors {includeVariants
          ? 'border-indigo-300 bg-indigo-50/40 dark:border-indigo-700 dark:bg-indigo-900/20'
          : 'border-gray-100 hover:border-gray-200 dark:border-gray-800 dark:hover:border-gray-700'}"
      >
        <input
          type="checkbox"
          bind:checked={includeVariants}
          class="mt-0.5 accent-indigo-600"
        />
        <div>
          <p class="text-sm font-medium text-gray-900 dark:text-gray-100">
            Include variants
          </p>
          <Hint class="text-xs"
            >Export generated and uploaded variants alongside originals</Hint
          >
        </div>
      </label>
    {:else}
      <!-- Destination type -->
      <div>
        <p class="text-md mb-2 font-medium text-gray-700 dark:text-gray-300">
          Destination
        </p>
        <DestinationTypePicker
          bind:value={destType}
          onchange={() => {
            validateResult = null
          }}
        />
      </div>

      <!-- Connection details -->
      <div>
        <p class="text-md mb-2 font-medium text-gray-700 dark:text-gray-300">
          Connection details
        </p>
        {#if destType === 'sftp'}
          <SftpConfigForm bind:value={sftpConfig} errors={fieldErrors} />
        {:else}
          <GDriveConfigForm bind:value={gdriveConfig} errors={fieldErrors} />
        {/if}
      </div>

      <!-- Test connection -->
      <div
        class="rounded-lg border border-gray-100 bg-gray-50 p-4 dark:border-gray-800 dark:bg-gray-800/50"
      >
        <div class="flex items-center justify-between gap-3">
          <div class="text-sm text-gray-600 dark:text-gray-300">
            {#if validateResult === null && !validating}
              <Hint class="text-sm">Verify credentials before saving</Hint>
            {:else if validateResult === 'ok'}
              <span
                class="flex items-center gap-1.5 text-green-700 dark:text-green-400"
              >
                <CheckCircle class="size-4 shrink-0" /> Connected
              </span>
            {:else if validateResult === 'error'}
              <span
                class="flex items-center gap-1.5 text-red-600 dark:text-red-400"
              >
                <AlertCircle class="size-4 shrink-0" />
                {validateError}
              </span>
            {/if}
          </div>
          <Button
            variant="secondary"
            size="sm"
            loading={validating}
            onclick={validate}
          >
            Test connection
          </Button>
        </div>
      </div>

      <!-- Schedule -->
      <div>
        <p class="text-md mb-2 font-medium text-gray-700 dark:text-gray-300">
          Schedule
        </p>
        <div class="space-y-2">
          <label
            class="flex cursor-pointer items-center gap-3 rounded-lg border p-3 transition-colors {scheduleType ===
            'manual'
              ? 'border-indigo-300 bg-indigo-50/40 dark:border-indigo-700 dark:bg-indigo-900/20'
              : 'border-gray-100 hover:border-gray-200 dark:border-gray-800 dark:hover:border-gray-700'}"
          >
            <input
              type="radio"
              bind:group={scheduleType}
              value="manual"
              class="accent-indigo-600"
            />
            <p class="text-sm font-medium text-gray-900 dark:text-gray-100">
              Manual only
            </p>
          </label>
          <label
            class="flex cursor-pointer items-start gap-3 rounded-lg border p-3 transition-colors {scheduleType ===
            'after_quiet'
              ? 'border-indigo-300 bg-indigo-50/40 dark:border-indigo-700 dark:bg-indigo-900/20'
              : 'border-gray-100 hover:border-gray-200 dark:border-gray-800 dark:hover:border-gray-700'}"
          >
            <input
              type="radio"
              bind:group={scheduleType}
              value="after_quiet"
              class="mt-0.5 accent-indigo-600"
            />
            <div class="flex flex-wrap items-center gap-2">
              <span class="text-sm font-medium text-gray-900 dark:text-gray-100"
                >After</span
              >
              <input
                type="number"
                class="w-16 rounded-lg border border-gray-300 bg-white px-2 py-1.5 text-sm text-gray-900 shadow-sm focus:border-indigo-400 focus:ring-2 focus:ring-indigo-200 focus:outline-none disabled:opacity-50 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
                min="1"
                max="999"
                bind:value={quietValue}
                disabled={scheduleType !== 'after_quiet'}
              />
              <select
                class="rounded-lg border border-gray-300 bg-white px-2 py-1.5 text-sm text-gray-900 shadow-sm focus:border-indigo-400 focus:ring-2 focus:ring-indigo-200 focus:outline-none disabled:opacity-50 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
                bind:value={quietUnit}
                disabled={scheduleType !== 'after_quiet'}
              >
                <option value="min">min</option>
                <option value="h">h</option>
              </select>
              <Hint class="text-sm">of quiet (no new uploads)</Hint>
            </div>
          </label>
        </div>
      </div>

      <!-- Label -->
      <div>
        <label
          for="export-name"
          class="text-md mb-1 block font-medium text-gray-700 dark:text-gray-300"
        >
          Label
        </label>
        <input
          id="export-name"
          class="text-md w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-gray-900 shadow-sm focus:border-indigo-400 focus:ring-2 focus:ring-indigo-200 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100 dark:focus:border-indigo-500 dark:focus:ring-indigo-900"
          bind:value={label}
          placeholder="e.g. Nightly SFTP backup"
        />
        <Hint class="mt-1 text-xs"
          >A short name to identify this export config</Hint
        >
        {#if fieldErrors.label}
          <p class="mt-1 text-xs text-red-500">{fieldErrors.label}</p>
        {/if}
      </div>

      {#if saveError}
        <Feedback error={saveError} class="mx-0" />
      {/if}
    {/if}
  </div>

  <!-- Footer -->
  <div
    class="flex items-center justify-between border-t border-gray-100 px-6 py-4 dark:border-gray-800"
  >
    <Button
      variant="secondary"
      onclick={() => {
        step = 1
        onClose()
      }}>Cancel</Button
    >

    <div class="flex gap-2">
      {#if step === 2}
        <Button variant="secondary" onclick={() => (step = 1)}>Back</Button>
      {/if}

      {#if step === 1}
        <Button
          variant="primary"
          disabled={!step1Valid()}
          onclick={() => (step = 2)}
        >
          Next
        </Button>
      {:else}
        <Button variant="primary" loading={saving} onclick={save}>
          {isEdit ? 'Save' : 'Create'}
        </Button>
      {/if}
    </div>
  </div>
</Modal>
