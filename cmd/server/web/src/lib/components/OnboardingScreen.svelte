<script lang="ts">
  import { Upload, FolderOpen, Tag, Share2 } from '@lucide/svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import { m } from '$lib/paraglide/messages'

  interface Props {
    onDismiss: () => void
  }

  let { onDismiss }: Props = $props()

  const cards = [
    {
      icon: Upload,
      iconClass:
        'bg-indigo-100 dark:bg-indigo-900/40 text-indigo-600 dark:text-indigo-400',
      title: m.onboard_upload(),
      body: m.onboard_upload_body(),
      hint: m.onboard_upload_hint(),
    },
    {
      icon: FolderOpen,
      iconClass:
        'bg-violet-100 dark:bg-violet-900/40 text-violet-600 dark:text-violet-400',
      title: m.onboard_project(),
      body: m.onboard_project_body(),
      hint: m.onboard_project_hint(),
    },
    {
      icon: Tag,
      iconClass:
        'bg-emerald-100 dark:bg-emerald-900/40 text-emerald-600 dark:text-emerald-400',
      title: m.onboard_tags(),
      body: m.onboard_tags_body(),
      hint: m.onboard_tags_hint(),
    },
    {
      icon: Share2,
      iconClass: 'bg-sky-100 dark:bg-sky-900/40 text-sky-600 dark:text-sky-400',
      title: m.onboard_share(),
      body: m.onboard_share_body(),
      hint: m.onboard_share_hint(),
    },
  ]
</script>

<div
  class="mx-auto flex max-w-3xl flex-col items-center justify-center px-8 py-16"
>
  <!-- Header -->
  <p
    class="text-md font-semibold tracking-widest text-indigo-500 uppercase dark:text-indigo-400"
  >
    {m.workspace_ready()}
  </p>
  <h1 class="mt-2 text-3xl font-bold text-gray-900 dark:text-gray-50">
    {m.onboard_start_building()}
  </h1>
  <p class="mt-2 max-w-md text-center text-lg text-gray-500 dark:text-gray-400">
    {m.onboard_what_to_do()}
  </p>

  <!-- Feature cards -->
  <div class="mt-8 grid w-full grid-cols-2 gap-4">
    {#each cards as card}
      {@const Icon = card.icon}
      <div
        class="flex flex-col gap-3 rounded-2xl border border-gray-200 bg-white p-5 transition-colors hover:border-indigo-300 dark:border-gray-700 dark:bg-gray-800/60 dark:hover:border-indigo-700"
      >
        <div class="flex items-center justify-start gap-3">
          <div
            class="flex h-9 w-9 items-center justify-center rounded-xl {card.iconClass}"
          >
            <Icon class="h-4 w-4" />
          </div>
          <p class="text-lg font-semibold text-gray-900 dark:text-gray-50">
            {card.title}
          </p>
        </div>
        <p class="text-lg leading-relaxed text-gray-500 dark:text-gray-400">
          {card.body}
        </p>
        <p
          class="text-md mt-auto border-t border-gray-100 pt-2 text-gray-400 dark:border-gray-700/60 dark:text-gray-500"
        >
          {card.hint}
        </p>
      </div>
    {/each}
  </div>

  <!-- Footer -->
  <div class="mt-10 flex items-center gap-4">
    <Button onclick={onDismiss}>{m.get_started()}</Button>
    <span class="text-md text-gray-400 dark:text-gray-500">
      Tip: press <kbd
        class="text-md rounded bg-gray-100 px-1.5 py-0.5 font-mono text-gray-500 dark:bg-gray-700 dark:text-gray-400"
        >⌘K</kbd
      > to quickly jump between projects
    </span>
  </div>
</div>
