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
    Upload,
    Download,
    Settings2,
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
  let addMenuEl: HTMLDivElement | undefined = $state()

  function handleClickOutside(e: MouseEvent) {
    if (addMenuEl && !addMenuEl.contains(e.target as Node)) {
      addMenuOpen = false
    }
  }

  $effect(() => {
    if (addMenuOpen) {
      document.addEventListener('click', handleClickOutside, { capture: true })
      return () =>
        document.removeEventListener('click', handleClickOutside, {
          capture: true,
        })
    }
  })

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
      <div class="add-btn-group relative flex" bind:this={addMenuEl}>
        <!-- Primary: upload files -->
        <label class="add-btn-primary">
          <Plus class="add-plus-icon h-4 w-4" />
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

        <!-- Chevron toggle -->
        <button
          type="button"
          class="add-btn-chevron"
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
          <ChevronDown
            class="chevron-icon h-4 w-4 {addMenuOpen ? 'open' : ''}"
          />
        </button>

        <!-- Dropdown -->
        {#if addMenuOpen}
          <div
            role="menu"
            tabindex="-1"
            class="add-dropdown"
            onkeydown={(e) => {
              if (e.key === 'Escape') {
                addMenuOpen = false
              }
            }}
          >
            <!-- Upload -->
            <label class="add-dropdown-item">
              <Upload class="add-dropdown-icon" />
              {m.upload_files()}
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
                  addMenuOpen = false
                }}
              />
            </label>

            <div class="add-dropdown-divider"></div>

            <!-- Workspace actions -->
            <p class="add-dropdown-group-label">Workspace</p>
            <a
              href="/library/settings/ingress"
              role="menuitem"
              class="add-dropdown-item"
              onclick={() => {
                addMenuOpen = false
              }}
            >
              <Download class="add-dropdown-icon" />
              {m.add_ingress_source()}
            </a>
            <a
              href="/library/settings/members"
              role="menuitem"
              class="add-dropdown-item"
              onclick={() => {
                addMenuOpen = false
              }}
            >
              <Users class="add-dropdown-icon" />
              {m.member_invite()}
            </a>

            <div class="add-dropdown-divider"></div>

            <!-- Schema actions -->
            <p class="add-dropdown-group-label">Schema</p>
            <a
              href="/library/settings/tags"
              role="menuitem"
              class="add-dropdown-item"
              onclick={() => {
                addMenuOpen = false
              }}
            >
              <Tags class="add-dropdown-icon" />
              {m.tags_manage()}
            </a>
            <a
              href="/library/settings/custom-fields"
              role="menuitem"
              class="add-dropdown-item"
              onclick={() => {
                addMenuOpen = false
              }}
            >
              <Settings2 class="add-dropdown-icon" />
              {m.fields_manage()}
            </a>
          </div>
        {/if}
      </div>
    {/if}
  </div>
</header>

<style>
  /* ── Split button shell ── */
  .add-btn-group {
    display: flex;
    border-radius: 8px;
    box-shadow: 0 1px 2px 0 rgb(0 0 0 / 0.08);
  }

  /* ── Primary upload label ── */
  .add-btn-primary {
    display: flex;
    cursor: pointer;
    align-items: center;
    gap: 6px;
    border-radius: 8px 0 0 8px;
    background: var(--accent-cta);
    padding: 7px 13px;
    font-size: 0.875rem;
    font-weight: 500;
    color: #fff;
    transition:
      background 0.15s ease,
      transform 0.08s ease;
    user-select: none;
  }
  .add-btn-primary:hover {
    background: var(--accent-cta-hover);
  }
  .add-btn-primary:active {
    transform: translateY(1px);
    background: var(--accent-cta-active);
  }

  /* Plus icon: subtle spin on hover to signal "adding" */
  .add-plus-icon {
    transition: transform 0.2s cubic-bezier(0.25, 1, 0.5, 1);
  }
  .add-btn-primary:hover .add-plus-icon {
    transform: rotate(90deg);
  }

  /* ── Chevron toggle ── */
  .add-btn-chevron {
    display: flex;
    align-items: center;
    border-radius: 0 8px 8px 0;
    border-left: 1px solid rgba(255 255 255 / 0.18);
    background: var(--accent-cta);
    padding: 7px 8px;
    color: #fff;
    transition:
      background 0.15s ease,
      transform 0.08s ease;
    cursor: pointer;
  }
  .add-btn-chevron:hover {
    background: var(--accent-cta-hover);
  }
  .add-btn-chevron:active {
    transform: translateY(1px);
  }

  /* Chevron rotates when menu is open */
  .chevron-icon {
    transition: transform 0.2s cubic-bezier(0.25, 1, 0.5, 1);
  }
  .chevron-icon.open {
    transform: rotate(180deg);
  }

  /* ── Dropdown panel ── */
  .add-dropdown {
    position: absolute;
    top: calc(100% + 6px);
    right: 0;
    z-index: 50;
    min-width: 220px;
    border-radius: 10px;
    border: 1px solid var(--border);
    background: var(--bg-surface);
    padding: 4px 0;
    box-shadow:
      0 0 0 1px rgb(0 0 0 / 0.04),
      0 4px 8px -2px rgb(0 0 0 / 0.12),
      0 16px 32px -8px rgb(0 0 0 / 0.18);
    animation: dropdown-in 0.14s cubic-bezier(0.25, 1, 0.5, 1) both;
    transform-origin: top right;
  }

  :global(.dark) .add-dropdown {
    border-color: var(--border);
    background: var(--bg-surface);
    box-shadow:
      0 0 0 1px rgb(0 0 0 / 0.3),
      0 4px 8px -2px rgb(0 0 0 / 0.4),
      0 16px 32px -8px rgb(0 0 0 / 0.5);
  }

  @keyframes dropdown-in {
    from {
      opacity: 0;
      transform: scale(0.95) translateY(-4px);
    }
    to {
      opacity: 1;
      transform: scale(1) translateY(0);
    }
  }

  /* Group label */
  .add-dropdown-group-label {
    padding: 8px 12px 3px;
    font-size: 0.6875rem;
    font-weight: 500;
    letter-spacing: 0.04em;
    text-transform: uppercase;
    color: var(--text-muted);
  }

  /* Divider */
  .add-dropdown-divider {
    margin: 4px 0;
    height: 1px;
    background: var(--border-subtle);
  }

  :global(.dark) .add-dropdown-divider {
    background: var(--bg-elevated);
  }

  /* Item row */
  .add-dropdown-item {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 6px 12px;
    font-size: 0.8125rem;
    font-weight: 500;
    color: var(--text-primary);
    text-decoration: none;
    transition: background 0.08s ease;
    cursor: pointer;
  }
  .add-dropdown-item:hover {
    background: var(--bg-hover);
  }

  :global(.dark) .add-dropdown-item:hover {
    background: var(--bg-elevated);
  }

  /* Inline icon */
  :global(.add-dropdown-icon) {
    width: 14px;
    height: 14px;
    flex-shrink: 0;
    color: var(--text-muted);
    transition: color 0.08s ease;
  }
  .add-dropdown-item:hover :global(.add-dropdown-icon) {
    color: var(--accent-cta);
  }
</style>
