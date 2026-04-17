<script lang="ts">
  import { Eye, EyeOff } from '@lucide/svelte'
  import type { IngressSourceType } from '$lib/api/models'
  import Input from '$lib/components/ui/Input.svelte'
  import { m } from '$lib/paraglide/messages'

  interface Props {
    type: IngressSourceType
    config: Record<string, unknown>
  }

  let { type, config = $bindable({}) }: Props = $props()

  // Helpers — two-way binding per field via get/set
  function field(key: string): string {
    return (config[key] as string) ?? ''
  }
  function setField(key: string, value: unknown) {
    config = { ...config, [key]: value }
  }
  function boolField(key: string, def = false): boolean {
    return key in config ? Boolean(config[key]) : def
  }

  // Password visibility toggles
  let showPassword = $state(false)
  let showSecretKey = $state(false)
</script>

{#if type === 'email_api'}
  <!-- No config needed — handled by IV-5 UI -->
  <div class="rounded-lg border border-indigo-100 bg-indigo-50/50 p-4 text-md text-indigo-700 dark:border-indigo-900/50 dark:bg-indigo-900/20 dark:text-indigo-300">
    No configuration needed. After saving, you'll get a unique ingest email address.
  </div>

{:else if type === 'imap'}
  <div class="space-y-4">
    <div class="grid grid-cols-2 gap-4">
      <Input
        label="IMAP host"
        placeholder="imap.gmail.com"
        value={field('host')}
        oninput={(e) => setField('host', (e.target as HTMLInputElement).value)}
      />
      <Input
        label="Port"
        type="number"
        placeholder="993"
        value={field('port') || '993'}
        oninput={(e) => setField('port', Number((e.target as HTMLInputElement).value))}
      />
    </div>
    <div class="flex items-center gap-2">
      <input
        id="imap-tls"
        type="checkbox"
        checked={boolField('tls', true)}
        onchange={(e) => setField('tls', (e.target as HTMLInputElement).checked)}
        class="h-4 w-4 rounded border-gray-300 text-indigo-600 focus:ring-indigo-500"
      />
      <label for="imap-tls" class="text-md text-gray-700 dark:text-gray-300">Use TLS</label>
    </div>
    <Input
      label="Username"
      placeholder="you@gmail.com"
      value={field('username')}
      oninput={(e) => setField('username', (e.target as HTMLInputElement).value)}
    />
    <div class="relative">
      <Input
        label="Password / App password"
        type={showPassword ? 'text' : 'password'}
        placeholder="••••••••"
        value={field('password')}
        oninput={(e) => setField('password', (e.target as HTMLInputElement).value)}
      />
      <button
        type="button"
        class="absolute right-3 top-8 text-gray-400 hover:text-gray-600 dark:hover:text-gray-200"
        onclick={() => { showPassword = !showPassword }}
        tabindex="-1"
      >
        {#if showPassword}<EyeOff class="h-4 w-4" />{:else}<Eye class="h-4 w-4" />{/if}
      </button>
    </div>
    <Input
      label="Mailbox"
      placeholder="INBOX"
      value={field('mailbox') || 'INBOX'}
      oninput={(e) => setField('mailbox', (e.target as HTMLInputElement).value)}
    />
    <div>
      <label for="imap-after-import" class="mb-1 block text-md font-medium text-gray-700 dark:text-gray-300">After import</label>
      <select
        id="imap-after-import"
        value={field('after_import') || 'mark_read'}
        onchange={(e) => setField('after_import', (e.target as HTMLSelectElement).value)}
        class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-md text-gray-900 focus:border-indigo-400 focus:outline-none focus:ring-2 focus:ring-indigo-200 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
      >
        <option value="mark_read">Mark as read</option>
        <option value="move_to">Move to mailbox</option>
        <option value="delete">{m.delete()}</option>
      </select>
    </div>
    {#if field('after_import') === 'move_to'}
      <Input
        label="Move to mailbox"
        placeholder="Imported"
        value={field('move_to_mailbox')}
        oninput={(e) => setField('move_to_mailbox', (e.target as HTMLInputElement).value)}
      />
    {/if}
  </div>

{:else if type === 'sftp'}
  <div class="space-y-4">
    <div class="grid grid-cols-2 gap-4">
      <Input
        label="Host"
        placeholder="files.client.com"
        value={field('host')}
        oninput={(e) => setField('host', (e.target as HTMLInputElement).value)}
      />
      <Input
        label="Port"
        type="number"
        placeholder="22"
        value={field('port') || '22'}
        oninput={(e) => setField('port', Number((e.target as HTMLInputElement).value))}
      />
    </div>
    <Input
      label="Username"
      placeholder="uploader"
      value={field('username')}
      oninput={(e) => setField('username', (e.target as HTMLInputElement).value)}
    />
    <div>
      <label for="sftp-auth-method" class="mb-1 block text-md font-medium text-gray-700 dark:text-gray-300">Auth method</label>
      <select
        id="sftp-auth-method"
        value={field('auth_method') || 'password'}
        onchange={(e) => setField('auth_method', (e.target as HTMLSelectElement).value)}
        class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-md text-gray-900 focus:border-indigo-400 focus:outline-none focus:ring-2 focus:ring-indigo-200 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
      >
        <option value="password">Password</option>
        <option value="key">Private key</option>
      </select>
    </div>
    {#if field('auth_method') !== 'key'}
      <div class="relative">
        <Input
          label="Password"
          type={showPassword ? 'text' : 'password'}
          placeholder="••••••••"
          value={field('password')}
          oninput={(e) => setField('password', (e.target as HTMLInputElement).value)}
        />
        <button
          type="button"
          class="absolute right-3 top-8 text-gray-400 hover:text-gray-600 dark:hover:text-gray-200"
          onclick={() => { showPassword = !showPassword }}
          tabindex="-1"
        >
          {#if showPassword}<EyeOff class="h-4 w-4" />{:else}<Eye class="h-4 w-4" />{/if}
        </button>
      </div>
    {:else}
      <div class="relative">
        <label for="sftp-private-key" class="mb-1 block text-md font-medium text-gray-700 dark:text-gray-300">Private key (PEM)</label>
        <textarea
          id="sftp-private-key"
          rows="4"
          placeholder="-----BEGIN OPENSSH PRIVATE KEY-----&#10;…"
          value={field('private_key')}
          oninput={(e) => setField('private_key', (e.target as HTMLTextAreaElement).value)}
          class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 font-mono text-sm text-gray-900 shadow-sm focus:border-indigo-400 focus:outline-none focus:ring-2 focus:ring-indigo-200 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
        ></textarea>
      </div>
    {/if}
    <Input
      label="Remote path"
      placeholder="/uploads/incoming"
      value={field('remote_path')}
      oninput={(e) => setField('remote_path', (e.target as HTMLInputElement).value)}
    />
    <div>
      <label for="sftp-after-import" class="mb-1 block text-md font-medium text-gray-700 dark:text-gray-300">After import</label>
      <select
        id="sftp-after-import"
        value={field('after_import') || 'leave'}
        onchange={(e) => setField('after_import', (e.target as HTMLSelectElement).value)}
        class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-md text-gray-900 focus:border-indigo-400 focus:outline-none focus:ring-2 focus:ring-indigo-200 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
      >
        <option value="leave">Leave in place</option>
        <option value="move_to">Move to path</option>
        <option value="delete">{m.delete()}</option>
      </select>
    </div>
    {#if field('after_import') === 'move_to'}
      <Input
        label="Move to path"
        placeholder="/uploads/done"
        value={field('move_to_path')}
        oninput={(e) => setField('move_to_path', (e.target as HTMLInputElement).value)}
      />
    {/if}
  </div>

{:else if type === 'dav'}
  <div class="space-y-4">
    <Input
      label="WebDAV URL"
      placeholder="https://cloud.example.com/remote.php/dav/files/user/Uploads/"
      value={field('url')}
      oninput={(e) => setField('url', (e.target as HTMLInputElement).value)}
    />
    <p class="text-sm text-gray-400 dark:text-gray-500">
      Nextcloud: <code class="rounded bg-gray-100 px-1 dark:bg-gray-800">{'{nextcloud_url}'}/remote.php/dav/files/{'{username}'}/</code>
    </p>
    <Input
      label="Username"
      placeholder="user"
      value={field('username')}
      oninput={(e) => setField('username', (e.target as HTMLInputElement).value)}
    />
    <div class="relative">
      <Input
        label="Password / App password"
        type={showPassword ? 'text' : 'password'}
        placeholder="••••••••"
        value={field('password')}
        oninput={(e) => setField('password', (e.target as HTMLInputElement).value)}
      />
      <button
        type="button"
        class="absolute right-3 top-8 text-gray-400 hover:text-gray-600 dark:hover:text-gray-200"
        onclick={() => { showPassword = !showPassword }}
        tabindex="-1"
      >
        {#if showPassword}<EyeOff class="h-4 w-4" />{:else}<Eye class="h-4 w-4" />{/if}
      </button>
    </div>
    <div>
      <label for="dav-after-import" class="mb-1 block text-md font-medium text-gray-700 dark:text-gray-300">After import</label>
      <select
        id="dav-after-import"
        value={field('after_import') || 'leave'}
        onchange={(e) => setField('after_import', (e.target as HTMLSelectElement).value)}
        class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-md text-gray-900 focus:border-indigo-400 focus:outline-none focus:ring-2 focus:ring-indigo-200 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
      >
        <option value="leave">Leave in place</option>
        <option value="move_to">Move to URL</option>
        <option value="delete">{m.delete()}</option>
      </select>
    </div>
    {#if field('after_import') === 'move_to'}
      <Input
        label="Move to WebDAV URL"
        placeholder="https://cloud.example.com/remote.php/dav/files/user/Done/"
        value={field('move_to_url')}
        oninput={(e) => setField('move_to_url', (e.target as HTMLInputElement).value)}
      />
    {/if}
  </div>

{:else if type === 's3'}
  <div class="space-y-4">
    <Input
      label="Endpoint (leave blank for AWS)"
      placeholder="https://s3.amazonaws.com"
      value={field('endpoint')}
      oninput={(e) => setField('endpoint', (e.target as HTMLInputElement).value)}
    />
    <div class="grid grid-cols-2 gap-4">
      <Input
        label="Region"
        placeholder="us-east-1"
        value={field('region')}
        oninput={(e) => setField('region', (e.target as HTMLInputElement).value)}
      />
      <Input
        label="Bucket"
        placeholder="client-uploads"
        value={field('bucket')}
        oninput={(e) => setField('bucket', (e.target as HTMLInputElement).value)}
      />
    </div>
    <Input
      label="Prefix (optional)"
      placeholder="incoming/"
      value={field('prefix')}
      oninput={(e) => setField('prefix', (e.target as HTMLInputElement).value)}
    />
    <Input
      label="Access key ID"
      placeholder="AKIA…"
      value={field('access_key_id')}
      oninput={(e) => setField('access_key_id', (e.target as HTMLInputElement).value)}
    />
    <div class="relative">
      <Input
        label="Secret access key"
        type={showSecretKey ? 'text' : 'password'}
        placeholder="••••••••"
        value={field('secret_access_key')}
        oninput={(e) => setField('secret_access_key', (e.target as HTMLInputElement).value)}
      />
      <button
        type="button"
        class="absolute right-3 top-8 text-gray-400 hover:text-gray-600 dark:hover:text-gray-200"
        onclick={() => { showSecretKey = !showSecretKey }}
        tabindex="-1"
      >
        {#if showSecretKey}<EyeOff class="h-4 w-4" />{:else}<Eye class="h-4 w-4" />{/if}
      </button>
    </div>
    <div>
      <label for="s3-after-import" class="mb-1 block text-md font-medium text-gray-700 dark:text-gray-300">After import</label>
      <select
        id="s3-after-import"
        value={field('after_import') || 'leave'}
        onchange={(e) => setField('after_import', (e.target as HTMLSelectElement).value)}
        class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-md text-gray-900 focus:border-indigo-400 focus:outline-none focus:ring-2 focus:ring-indigo-200 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
      >
        <option value="leave">Leave in place</option>
        <option value="move_to">Move to prefix</option>
        <option value="delete">{m.delete()}</option>
      </select>
    </div>
    {#if field('after_import') === 'move_to'}
      <Input
        label="Move to prefix"
        placeholder="done/"
        value={field('move_to_prefix')}
        oninput={(e) => setField('move_to_prefix', (e.target as HTMLInputElement).value)}
      />
    {/if}
    <div class="flex items-center gap-2">
      <input
        id="s3-path-style"
        type="checkbox"
        checked={boolField('use_path_style', false)}
        onchange={(e) => setField('use_path_style', (e.target as HTMLInputElement).checked)}
        class="h-4 w-4 rounded border-gray-300 text-indigo-600 focus:ring-indigo-500"
      />
      <label for="s3-path-style" class="text-md text-gray-700 dark:text-gray-300">
        Use path-style URLs (required for MinIO)
      </label>
    </div>
  </div>
{/if}
