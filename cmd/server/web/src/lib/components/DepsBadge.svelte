<script lang="ts">
  import {
    AlertTriangle,
    CheckCircle2,
    LoaderCircle,
    ExternalLink,
  } from '@lucide/svelte'
  import { m } from '$lib/paraglide/messages'
  import { openExternalURL } from '$lib/desktop'

  interface Props {
    name: string
    binary: string
    found: boolean
    version?: string
    features: string[]
    docsUrl: string
    checking?: boolean
  }

  let {
    name,
    binary,
    found,
    version = '',
    features,
    docsUrl,
    checking = false,
  }: Props = $props()

  let expanded = $state(false)
</script>

<article
  class="rounded-2xl border border-slate-200 bg-white/90 p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900/80"
>
  <div class="flex items-start justify-between gap-4">
    <div>
      <div class="flex items-center gap-2">
        <h3 class="font-semibold text-slate-900 dark:text-slate-100">{name}</h3>
        <code
          class="rounded bg-slate-100 px-2 py-0.5 text-xs text-slate-600 dark:bg-slate-800 dark:text-slate-300"
        >
          {binary}
        </code>
      </div>
      {#if found && version}
        <p class="mt-2 text-sm text-emerald-700 dark:text-emerald-400">
          {version}
        </p>
      {/if}
    </div>

    {#if checking}
      <div class="flex items-center gap-2 text-amber-600 dark:text-amber-400">
        <LoaderCircle class="h-4 w-4 animate-spin" />
        <span class="text-sm">{m.setup_deps_checking()}</span>
      </div>
    {:else if found}
      <div
        class="flex items-center gap-2 text-emerald-600 dark:text-emerald-400"
      >
        <CheckCircle2 class="h-4 w-4" />
        <span class="text-sm">{m.setup_deps_found()}</span>
      </div>
    {:else}
      <div class="flex items-center gap-2 text-amber-600 dark:text-amber-400">
        <AlertTriangle class="h-4 w-4" />
        <span class="text-sm">{m.setup_deps_missing()}</span>
      </div>
    {/if}
  </div>

  {#if !checking && !found}
    <div
      class="mt-4 rounded-xl bg-amber-50 p-4 text-sm text-amber-950 dark:bg-amber-500/10 dark:text-amber-100"
    >
      <button
        class="font-medium underline decoration-dotted underline-offset-4"
        onclick={() => (expanded = !expanded)}
      >
        {m.setup_deps_features_disabled()}
      </button>

      {#if expanded}
        <ul class="mt-3 list-disc space-y-1 pl-5">
          {#each features as feature}
            <li>{feature}</li>
          {/each}
        </ul>
      {/if}

      <button
        class="mt-4 inline-flex items-center gap-1 font-medium text-amber-800 underline underline-offset-4 dark:text-amber-300"
        onclick={() => openExternalURL(docsUrl)}
      >
        {m.setup_deps_install()}
        <ExternalLink class="h-3.5 w-3.5" />
      </button>
    </div>
  {/if}
</article>
