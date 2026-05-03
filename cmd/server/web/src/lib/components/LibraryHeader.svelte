<script lang="ts">
  import { authStore } from '$lib/stores/auth.svelte'
  import { assetsStore } from '$lib/stores/assets.svelte'
  import { projectsStore } from '$lib/stores/projects.svelte'
  import { collectionsStore } from '$lib/stores/collections.svelte'
  import SearchInput from '$lib/components/ui/SearchInput.svelte'
  import { navigationStore } from '$lib/stores/navigation.svelte'
  import {
    Share2,
    ChevronDown,
    Database,
    Users,
    Plus,
    Settings,
    Tags,
  } from '@lucide/svelte'
  import { m } from '$lib/paraglide/messages.js'
  import UndoRedo from './UndoRedo.svelte'
  import type { Snippet } from 'svelte'

  type Props = {
    prefix?: Snippet
    onShareProject?: () => void
    showShareButton?: boolean
  }

  let { onShareProject, showShareButton = false, prefix }: Props = $props()

  let addMenuOpen = $state(false)

  const activeCollection = $derived(
    navigationStore.activeCollectionId
      ? (collectionsStore.collections.find(
          (c) => c.id === navigationStore.activeCollectionId
        ) ?? null)
      : null
  )
</script>

<header
  class="flex items-center gap-4 border-b border-[var(--border-subtle)] bg-[var(--bg-surface)] px-4 py-3"
>
  <!-- LEFT: breadcrumb / title -->
  <div class="flex shrink-0 items-center gap-2">
    {@render prefix?.()}
    <div>
      <p class="text-sm font-semibold text-[var(--text-primary)]">
        {projectsStore.activeProjectName ?? m.library()}
      </p>
      <p class="text-xs text-[var(--text-muted)]">
        {#if activeCollection}{m.collection_id({
            id: activeCollection.name,
          })}{:else if projectsStore.activeProjectName}{m.project_id({
            name: projectsStore.activeProjectName,
          })}{:else}{m.all_assets()}{/if}
      </p>
    </div>
  </div>

  <!-- CENTER: search -->
  <div class="mx-auto w-full max-w-md flex-1">
    <SearchInput
      value={assetsStore.query}
      placeholder={m.search_anything()}
      onchange={(q) => {
        assetsStore.search(q)
      }}
    />
  </div>

  <!-- RIGHT: secondary actions + primary CTA -->
  <div class="flex shrink-0 items-center gap-2">
    <UndoRedo />

    {#if showShareButton}
      <button
        type="button"
        class="flex items-center rounded-lg p-2 text-[var(--text-muted)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-secondary)]"
        onclick={onShareProject}
        title={m.share()}
        aria-label={m.share()}
      >
        <Share2 class="h-4 w-4" />
      </button>
    {/if}

    {#if authStore.role !== 'viewer'}
      <div class="relative flex">
        <label
          class="flex cursor-pointer items-center gap-2 rounded-l-lg bg-indigo-600 px-3 py-2 text-sm font-medium text-white hover:bg-indigo-700"
        >
          <Plus class="h-4 w-4" />
          {m.add()}
          <input
            type="file"
            multiple
            data-upload-trigger
            class="hidden"
            onchange={(e) => {
              const files = Array.from(
                (e.target as HTMLInputElement).files ?? []
              )
              assetsStore.upload(
                files,
                navigationStore.activeProjectId,
                navigationStore.activeFolderId
              )
              ;(e.target as HTMLInputElement).value = ''
            }}
          />
        </label>
        <button
          type="button"
          class="flex items-center rounded-r-lg border-l border-indigo-500 bg-indigo-600 px-2 py-2 text-white hover:bg-indigo-700"
          onclick={() => {
            addMenuOpen = !addMenuOpen
          }}
          onkeydown={(e) => {
            if (e.key === 'Escape') addMenuOpen = false
          }}
          aria-label="More add options"
          aria-expanded={addMenuOpen}
          aria-haspopup="menu"
        >
          <ChevronDown class="h-4 w-4" />
        </button>
        {#if addMenuOpen}
          <!-- svelte-ignore a11y_no_static_element_interactions -->
          <div
            role="menu"
            tabindex="-1"
            class="absolute top-full right-0 z-50 mt-1 min-w-[200px] rounded-lg border border-[var(--border-default)] bg-[var(--bg-surface)] py-1 shadow-xl"
            onmouseleave={() => {
              addMenuOpen = false
            }}
            onkeydown={(e) => {
              if (e.key === 'Escape') addMenuOpen = false
            }}
          >
            <a
              href="/library/settings/ingress"
              class="flex items-center gap-2.5 px-3 py-2 text-sm text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]"
            >
              <Database class="h-4 w-4 shrink-0 text-[var(--text-muted)]" />
              {m.add_ingress_source()}
            </a>
            <a
              href="/library/settings/members"
              class="flex items-center gap-2.5 px-3 py-2 text-sm text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]"
            >
              <Users class="h-4 w-4 shrink-0 text-[var(--text-muted)]" />
              {m.member_invite()}
            </a>
            <a
              href="/library/settings/tags"
              class="flex items-center gap-2.5 px-3 py-2 text-sm text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]"
            >
              <Tags class="h-4 w-4 shrink-0 text-[var(--text-muted)]" />
              {m.tags_manage()}
            </a>
            <a
              href="/library/settings/custom-fields"
              class="flex items-center gap-2.5 px-3 py-2 text-sm text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]"
            >
              <Settings class="h-4 w-4 shrink-0 text-[var(--text-muted)]" />
              {m.fields_manage()}
            </a>
          </div>
        {/if}
      </div>
    {/if}
  </div>
</header>
