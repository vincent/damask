<script lang="ts">
  import { Mail, Server, HardDrive, Cloud, Inbox } from '@lucide/svelte'
  import type { IngressSourceType } from '$lib/api/models'

  interface Props {
    type: IngressSourceType
    class?: string
  }

  let { type, class: cls = 'h-5 w-5' }: Props = $props()
</script>

{#if type === 'email_api'}
  <Mail class={cls} />
{:else if type === 'imap'}
  <Inbox class={cls} />
{:else if type === 'sftp'}
  <Server class={cls} />
{:else if type === 'dav'}
  <HardDrive class={cls} />
{:else if type === 's3'}
  <Cloud class={cls} />
{/if}
