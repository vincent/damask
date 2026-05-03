<script lang="ts">
  import { authStore } from '$lib/stores/auth.svelte'
  import { assetsStore } from '$lib/stores/assets.svelte'
  import { projectsStore } from '$lib/stores/projects.svelte'
  import { collectionsStore } from '$lib/stores/collections.svelte'
  import SortButtons from '$lib/components/SortButtons.svelte'
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
  import Button from './ui/Button.svelte'
  import Title from './ui/Title.svelte'
  import type { Snippet } from 'svelte'
  import Hint from './ui/Hint.svelte'

  type Props = {
    prefix?: Snippet
    sort: 'mimetype' | 'created_at' | 'size' | 'taken_at'
    asc: boolean
    onShareProject?: () => void
    showShareButton?: boolean
  }

  let {
    sort = $bindable(),
    asc = $bindable(),
    onShareProject,
    showShareButton = false,
    prefix,
  }: Props = $props()

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
  class="flex items-center justify-between border-b border-[var(--border-subtle)] bg-[var(--bg-surface)] px-6 py-4"
>
  <div class="flex items-center gap-3">
    {@render prefix?.()}
    <div>
      <Title>{projectsStore.activeProjectName ?? m.library()}</Title>
      <Hint>
        {#if activeCollection}{m.collection_id({
            id: activeCollection.name,
          })}{:else if projectsStore.activeProjectName}{m.project_id({
            name: projectsStore.activeProjectName,
          })}{:else}{m.all_assets()}{/if}
      </Hint>
    </div>
    {#if showShareButton}
      <Button
        size="md"
        variant="outline"
        class="ms-3 flex items-center gap-1.5"
        onclick={onShareProject}
        title="Share this project"
      >
        <Share2 class="h-3.5 w-3.5" />
        {m.share()}
      </Button>
    {/if}
  </div>

  <div class="flex items-center gap-2">
    <SortButtons
      sort={(key, a) => assetsStore.sort(key, a)}
      bind:value={sort}
      bind:asc
      keys={{
        created_at: m.sort_date(),
        mimetype: m.sort_mimetype(),
        size: m.sort_weight(),
        taken_at: m.sort_date_taken(),
      }}
    />
  </div>

  <div class="flex items-center gap-2">
    <UndoRedo />

    <SearchInput
      class="w-64"
      value={assetsStore.query}
      placeholder={m.search_anything()}
      onchange={(q) => {
        assetsStore.search(q)
      }}
    />

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
          aria-label="More add options"
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
