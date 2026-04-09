<script lang="ts">
  import { Upload, FolderOpen, Tag, Share2 } from '@lucide/svelte'
  import Button from '$lib/components/ui/Button.svelte'

  interface Props {
    onDismiss: () => void
  }

  let { onDismiss }: Props = $props()

  const cards = [
    {
      icon: Upload,
      iconClass: 'bg-indigo-100 dark:bg-indigo-900/40 text-indigo-600 dark:text-indigo-400',
      title: 'Upload files',
      body: 'Drag files anywhere onto this page, or click the upload button in the header.',
      hint: 'Thumbnails are generated automatically.',
    },
    {
      icon: FolderOpen,
      iconClass: 'bg-violet-100 dark:bg-violet-900/40 text-violet-600 dark:text-violet-400',
      title: 'Create a project',
      body: 'Use the sidebar on the left to create a project and group related assets together.',
      hint: 'Add folders inside projects for deeper structure.',
    },
    {
      icon: Tag,
      iconClass: 'bg-emerald-100 dark:bg-emerald-900/40 text-emerald-600 dark:text-emerald-400',
      title: 'Tag & search',
      body: 'Add tags to assets to find them across projects. Use the search bar at the top to filter instantly.',
      hint: 'Select multiple assets with Shift+click to tag in bulk.',
    },
    {
      icon: Share2,
      iconClass: 'bg-sky-100 dark:bg-sky-900/40 text-sky-600 dark:text-sky-400',
      title: 'Share a project',
      body: 'Generate a read-only share link for any project or asset — no login required for viewers.',
      hint: 'Find the share button (↗) next to any project name.',
    },
  ]
</script>

<div class="flex flex-col items-center justify-center py-16 px-8 max-w-3xl mx-auto">
  <!-- Header -->
  <p class="text-md font-semibold uppercase tracking-widest text-indigo-500 dark:text-indigo-400">
    Your workspace is ready
  </p>
  <h1 class="mt-2 text-3xl font-bold text-gray-900 dark:text-gray-50">
    Start building your library
  </h1>
  <p class="mt-2 text-lg text-gray-500 dark:text-gray-400 max-w-md text-center">
    Here's what you can do right now.
  </p>

  <!-- Feature cards -->
  <div class="grid grid-cols-2 gap-4 mt-8 w-full">
    {#each cards as card}
      {@const Icon = card.icon}
      <div class="rounded-2xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800/60 p-5 flex flex-col gap-3 hover:border-indigo-300 dark:hover:border-indigo-700 transition-colors">
        <div class="flex items-center justify-start gap-3">
          <div class="w-9 h-9 rounded-xl flex items-center justify-center {card.iconClass}">
            <Icon class="h-4 w-4" />
          </div>
          <p class="text-lg font-semibold text-gray-900 dark:text-gray-50">{card.title}</p>
        </div>
        <p class="text-lg text-gray-500 dark:text-gray-400 leading-relaxed">{card.body}</p>
        <p class="text-md text-gray-400 dark:text-gray-500 mt-auto pt-2 border-t border-gray-100 dark:border-gray-700/60">
          {card.hint}
        </p>
      </div>
    {/each}
  </div>

  <!-- Footer -->
  <div class="mt-10 flex items-center gap-4">
    <Button onclick={onDismiss}>Get started</Button>
    <span class="text-md text-gray-400 dark:text-gray-500">
      Tip: press <kbd class="rounded bg-gray-100 dark:bg-gray-700 px-1.5 py-0.5 font-mono text-md text-gray-500 dark:text-gray-400">⌘K</kbd> to quickly jump between projects
    </span>
  </div>
</div>
