<script lang="ts">
  import type { Snippet } from 'svelte'
  import { goto } from '$app/navigation'
  import { m } from '$lib/paraglide/messages'
  import Button from '$lib/components/ui/Button.svelte'

  interface Props {
    title: string
    description?: string
    nextLabel?: string
    backHref?: string
    nextDisabled?: boolean
    loading?: boolean
    onNext: () => Promise<void> | void
    children?: Snippet
  }

  let {
    title,
    description,
    nextLabel = m.setup_next(),
    backHref,
    nextDisabled = false,
    loading = false,
    onNext,
    children,
  }: Props = $props()
</script>

<section
  class="rounded-[2rem] border border-white/60 bg-white/92 p-8 shadow-[0_24px_70px_rgba(15,23,42,0.12)] backdrop-blur dark:border-gray-800 dark:bg-gray-900/92"
>
  <header class="mb-8">
    <h1
      class="font-[Bricolage_Grotesque] text-3xl font-bold tracking-tight text-slate-900 dark:text-white"
    >
      {title}
    </h1>
    {#if description}
      <p
        class="mt-3 max-w-2xl text-sm leading-6 text-slate-600 dark:text-slate-300"
      >
        {description}
      </p>
    {/if}
  </header>

  <div class="space-y-5">
    {@render children?.()}
  </div>

  <footer
    class="mt-10 flex items-center justify-between border-t border-slate-200/80 pt-5 dark:border-slate-800"
  >
    <div>
      {#if backHref}
        <Button variant="ghost" onclick={() => goto(backHref)}>
          {m.setup_back()}
        </Button>
      {/if}
    </div>

    <Button onclick={() => onNext()} disabled={nextDisabled} {loading}>
      {nextLabel}
    </Button>
  </footer>
</section>
