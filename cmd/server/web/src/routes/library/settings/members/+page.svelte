<script lang="ts">
  import { onMount } from 'svelte'
  import { formatDistanceToNowStrict } from 'date-fns'
  import { UserPlus, Trash2, X } from '@lucide/svelte'
  import { workspaceApi } from '$lib/api'
  import { authStore } from '$lib/stores/auth.svelte'
  import { toastStore } from '$lib/stores/toast.svelte'
  import type { WorkspaceMember, WorkspaceInvite } from '$lib/api/models'
  import Spinner from '$lib/components/ui/Spinner.svelte'
  import EmptyState from '$lib/components/ui/EmptyState.svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import Badge from '$lib/components/ui/Badge.svelte'
  import Input from '$lib/components/ui/Input.svelte'
  import SectionHeading from '$lib/components/ui/SectionHeading.svelte'
  import Hint from '$lib/components/ui/Hint.svelte'

  const currentUserID = $derived(authStore.user?.id)
  const isOwner = $derived(authStore.role === 'owner')

  let members = $state<WorkspaceMember[]>([])
  let invites = $state<WorkspaceInvite[]>([])
  let loading = $state(true)
  let confirmRemoveID = $state<string | null>(null)

  let showInviteForm = $state(false)
  let inviteEmail = $state('')
  let inviteRole = $state<'editor' | 'viewer'>('editor')
  let inviting = $state(false)

  let updatingRole = $state<string | null>(null)
  let removingID = $state<string | null>(null)
  let deletingInviteID = $state<string | null>(null)

  async function load() {
    loading = true
    try {
      ;[members, invites] = await Promise.all([
        workspaceApi.listMembers(),
        workspaceApi.listInvites(),
      ])
    } catch (e) {
      toastStore.show(e instanceof Error ? e.message : 'Failed to load members', 'error')
    } finally {
      loading = false
    }
  }

  onMount(load)

  async function changeRole(userId: string, role: string) {
    updatingRole = userId
    try {
      await workspaceApi.updateMemberRole(userId, role)
      members = members.map(m => m.user_id === userId ? { ...m, role } : m)
      toastStore.show('Role updated')
    } catch (e) {
      toastStore.show(e instanceof Error ? e.message : 'Failed to update role', 'error')
    } finally {
      updatingRole = null
    }
  }

  async function removeMember(userId: string) {
    removingID = userId
    try {
      await workspaceApi.removeMember(userId)
      members = members.filter(m => m.user_id !== userId)
      toastStore.show('Member removed')
    } catch (e) {
      toastStore.show(e instanceof Error ? e.message : 'Failed to remove member', 'error')
    } finally {
      removingID = null
      confirmRemoveID = null
    }
  }

  async function cancelInvite(inviteId: string) {
    deletingInviteID = inviteId
    try {
      await workspaceApi.deleteInvite(inviteId)
      invites = invites.filter(i => i.id !== inviteId)
      toastStore.show('Invite cancelled')
    } catch (e) {
      toastStore.show(e instanceof Error ? e.message : 'Failed to cancel invite', 'error')
    } finally {
      deletingInviteID = null
    }
  }

  async function sendInvite() {
    if (!inviteEmail.trim()) return
    inviting = true
    try {
      await workspaceApi.createInvite(inviteEmail.trim(), inviteRole)
      toastStore.show(`Invite sent to ${inviteEmail.trim()}`)
      inviteEmail = ''
      inviteRole = 'editor'
      showInviteForm = false
      invites = await workspaceApi.listInvites()
    } catch (e) {
      toastStore.show(e instanceof Error ? e.message : 'Failed to send invite', 'error')
    } finally {
      inviting = false
    }
  }
</script>

<div class="flex-1 overflow-y-auto">
  <div class="mx-auto w-full max-w-2xl px-8 py-10 space-y-8">

    {#if !isOwner}
      <EmptyState
        title="Owner access required"
        description="Only workspace owners can manage members."
      />
    {:else if loading}
      <div class="flex justify-center py-12">
        <Spinner />
      </div>
    {:else}

      <!-- Members -->
      <section>
        <div class="flex-1">
          <div class="mb-3 flex items-center justify-between">
            <SectionHeading titleClass="text-xl" title="Members" count={members.length} />
            <Button variant="primary" onclick={() => { showInviteForm = !showInviteForm }}>
              <UserPlus class="mr-1.5 h-3.5 w-3.5" />
              Invite member
            </Button>
          </div>
          <Hint>Manage your workspace members and their permissions.</Hint>
        </div>
      </section>

      <section>
        {#if showInviteForm}
          <div class="mb-4 rounded-lg border border-zinc-200 bg-zinc-50 p-4 dark:border-zinc-700 dark:bg-zinc-800/50">
            <div class="flex gap-2">
              <Input
                type="email"
                placeholder="Email address"
                bind:value={inviteEmail}
                class="flex-1"
              />
              <select
                bind:value={inviteRole}
                class="rounded-md border border-zinc-300 bg-white px-3 py-1.5 text-sm text-zinc-700
                       focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500
                       dark:border-zinc-600 dark:bg-zinc-900 dark:text-zinc-200"
              >
                <option value="editor">Editor</option>
                <option value="viewer">Viewer</option>
              </select>
              <Button onclick={sendInvite} loading={inviting} disabled={!inviteEmail.trim()}>
                Send
              </Button>
              <button
                type="button"
                onclick={() => { showInviteForm = false }}
                class="rounded p-1.5 text-zinc-400 hover:text-zinc-600 dark:hover:text-zinc-200"
              >
                <X class="h-4 w-4" />
              </button>
            </div>
          </div>
        {/if}

        <div class="divide-y divide-zinc-100 rounded-lg border border-zinc-200 bg-white dark:divide-zinc-800 dark:border-zinc-700 dark:bg-zinc-900">
          {#each members as member (member.user_id)}
            <div class="flex items-center gap-3 px-4 py-3">
              <!-- Avatar -->
              <div class="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-indigo-100 text-xs font-semibold text-indigo-700 dark:bg-indigo-900/50 dark:text-indigo-300">
                {member.name.slice(0, 2).toUpperCase()}
              </div>

              <!-- Info -->
              <div class="min-w-0 flex-1">
                <div class="flex items-center gap-2">
                  <span class="truncate text-sm font-medium text-zinc-900 dark:text-zinc-100">{member.name}</span>
                  {#if member.user_id === currentUserID}
                    <span class="text-xs text-zinc-400">(you)</span>
                  {/if}
                </div>
                <div class="truncate text-xs text-zinc-500 dark:text-zinc-400">{member.email}</div>
              </div>

              <!-- Role -->
              {#if member.user_id === currentUserID || member.role === 'owner'}
                <Badge variant={member.role as 'owner' | 'editor' | 'viewer'}>{member.role}</Badge>
              {:else if updatingRole === member.user_id}
                <Spinner size="sm" />
              {:else}
                <select
                  value={member.role}
                  onchange={e => changeRole(member.user_id, (e.target as HTMLSelectElement).value)}
                  class="rounded border border-zinc-300 bg-white py-0.5 pl-2 pr-6 text-xs text-zinc-700
                         focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500
                         dark:border-zinc-600 dark:bg-zinc-800 dark:text-zinc-200"
                >
                  <option value="owner">Owner</option>
                  <option value="editor">Editor</option>
                  <option value="viewer">Viewer</option>
                </select>
              {/if}

              <!-- Remove -->
              {#if member.user_id !== currentUserID}
                {#if confirmRemoveID === member.user_id}
                  <div class="flex items-center gap-1.5">
                    <span class="text-xs text-zinc-500 dark:text-zinc-400">Remove?</span>
                    <button
                      type="button"
                      onclick={() => removeMember(member.user_id)}
                      disabled={removingID === member.user_id}
                      class="rounded px-2 py-0.5 text-xs font-medium text-red-600 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-900/30"
                    >
                      {removingID === member.user_id ? '…' : 'Yes'}
                    </button>
                    <button
                      type="button"
                      onclick={() => { confirmRemoveID = null }}
                      class="rounded px-2 py-0.5 text-xs text-zinc-500 hover:bg-zinc-100 dark:text-zinc-400 dark:hover:bg-zinc-800"
                    >
                      Cancel
                    </button>
                  </div>
                {:else}
                  <button
                    type="button"
                    onclick={() => { confirmRemoveID = member.user_id }}
                    class="rounded p-1 text-zinc-400 hover:text-red-500 dark:hover:text-red-400"
                    title="Remove member"
                  >
                    <Trash2 class="h-3.5 w-3.5" />
                  </button>
                {/if}
              {/if}
            </div>
          {/each}

          {#if members.length === 0}
            <div class="px-4 py-8 text-center text-sm text-zinc-400 dark:text-zinc-500">No members yet.</div>
          {/if}
        </div>
      </section>

      <!-- Pending invites -->
      {#if invites.length > 0}
        <section>
          <SectionHeading titleClass="text-xl"  title="Pending invites" count={invites.length} />
          <Hint>Manage pending member invitations.</Hint>
        </section>
        <section>
          <div class="divide-y divide-zinc-100 rounded-lg border border-zinc-200 bg-white dark:divide-zinc-800 dark:border-zinc-700 dark:bg-zinc-900">
            {#each invites as invite (invite.id)}
              <div class="flex items-center gap-3 px-4 py-3">
                <div class="min-w-0 flex-1">
                  <div class="truncate text-sm text-zinc-900 dark:text-zinc-100">{invite.email}</div>
                  <div class="text-xs text-zinc-500 dark:text-zinc-400">
                    Expires {formatDistanceToNowStrict(new Date(invite.expires_at), { addSuffix: true })}
                  </div>
                </div>
                <Badge variant={invite.role as 'editor' | 'viewer'}>{invite.role}</Badge>
                <button
                  type="button"
                  onclick={() => cancelInvite(invite.id)}
                  disabled={deletingInviteID === invite.id}
                  class="rounded p-1 text-zinc-400 hover:text-red-500 dark:hover:text-red-400 disabled:opacity-40"
                  title="Cancel invite"
                >
                  {#if deletingInviteID === invite.id}
                    <Spinner size="sm" />
                  {:else}
                    <X class="h-3.5 w-3.5" />
                  {/if}
                </button>
              </div>
            {/each}
          </div>
        </section>
      {/if}

    {/if}
  </div>
</div>
