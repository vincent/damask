<script lang="ts">
  import Input from '$lib/components/ui/Input.svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import { m } from '$lib/paraglide/messages'
  import type { StorageParams } from '$lib/api'
  import { pickDirectory } from '$lib/desktop'

  interface Props {
    value: StorageParams
    onChange: (value: StorageParams) => void
    validationError?: string
  }

  let { value, onChange, validationError }: Props = $props()

  function patch(patch: Partial<StorageParams>) {
    onChange({ ...value, ...patch })
  }

  async function chooseDirectory() {
    const path = await pickDirectory()
    if (path) patch({ localPath: path })
  }
</script>

<div class="space-y-5">
  <div class="grid gap-3 md:grid-cols-3">
    <label class:selected={value.type === 'local'} class="storage-option">
      <input
        type="radio"
        name="storage-type"
        checked={value.type === 'local'}
        onchange={() => patch({ type: 'local' })}
      />
      <span>{m.setup_storage_local()}</span>
    </label>
    <label class:selected={value.type === 's3'} class="storage-option">
      <input
        type="radio"
        name="storage-type"
        checked={value.type === 's3'}
        onchange={() => patch({ type: 's3' })}
      />
      <span>{m.setup_storage_s3()}</span>
    </label>
    <label class:selected={value.type === 'sftp'} class="storage-option">
      <input
        type="radio"
        name="storage-type"
        checked={value.type === 'sftp'}
        onchange={() => patch({ type: 'sftp' })}
      />
      <span>{m.setup_storage_sftp()}</span>
    </label>
  </div>

  {#if value.type === 'local'}
    <div class="space-y-3">
      <Input
        label={m.setup_storage_local_path()}
        value={value.localPath}
        error={validationError}
        oninput={(e) =>
          patch({ localPath: (e.currentTarget as HTMLInputElement).value })}
      />
      <div class="flex justify-end">
        <Button variant="secondary" onclick={chooseDirectory}>Browse</Button>
      </div>
    </div>
  {/if}

  {#if value.type === 's3'}
    <div class="grid gap-4 md:grid-cols-2">
      <Input
        label="Bucket"
        value={value.s3Bucket}
        oninput={(e) =>
          patch({ s3Bucket: (e.currentTarget as HTMLInputElement).value })}
        error={validationError}
      />
      <Input
        label="Region"
        value={value.s3Region}
        oninput={(e) =>
          patch({ s3Region: (e.currentTarget as HTMLInputElement).value })}
      />
      <Input
        label="Endpoint"
        value={value.s3Endpoint}
        oninput={(e) =>
          patch({ s3Endpoint: (e.currentTarget as HTMLInputElement).value })}
      />
      <Input
        label="Access key"
        value={value.s3AccessKey}
        oninput={(e) =>
          patch({ s3AccessKey: (e.currentTarget as HTMLInputElement).value })}
      />
      <Input
        label="Secret key"
        type="password"
        value={value.s3SecretKey}
        oninput={(e) =>
          patch({ s3SecretKey: (e.currentTarget as HTMLInputElement).value })}
      />
    </div>
  {/if}

  {#if value.type === 'sftp'}
    <div class="grid gap-4 md:grid-cols-2">
      <Input
        label="Host"
        value={value.sftpHost}
        oninput={(e) =>
          patch({ sftpHost: (e.currentTarget as HTMLInputElement).value })}
        error={validationError}
      />
      <Input
        label="Port"
        type="number"
        value={String(value.sftpPort)}
        oninput={(e) =>
          patch({
            sftpPort: Number((e.currentTarget as HTMLInputElement).value) || 0,
          })}
      />
      <Input
        label="User"
        value={value.sftpUser}
        oninput={(e) =>
          patch({ sftpUser: (e.currentTarget as HTMLInputElement).value })}
      />
      <Input
        label="SSH key path"
        value={value.sftpKeyPath}
        oninput={(e) =>
          patch({ sftpKeyPath: (e.currentTarget as HTMLInputElement).value })}
      />
      <Input
        label="Remote path"
        value={value.sftpRemotePath}
        oninput={(e) =>
          patch({
            sftpRemotePath: (e.currentTarget as HTMLInputElement).value,
          })}
      />
    </div>
  {/if}
</div>

<style>
  .storage-option {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    border: 1px solid rgb(203 213 225);
    border-radius: 1rem;
    padding: 1rem 1.1rem;
    background: rgb(255 255 255 / 0.7);
    cursor: pointer;
    transition:
      border-color 120ms ease,
      background-color 120ms ease,
      transform 120ms ease;
  }

  .storage-option:hover {
    border-color: rgb(14 116 144 / 0.45);
    transform: translateY(-1px);
  }

  .storage-option.selected {
    border-color: rgb(8 145 178);
    background: rgb(236 254 255);
  }

  .storage-option input {
    accent-color: rgb(8 145 178);
  }

  :global(.dark) .storage-option {
    border-color: rgb(51 65 85);
    background: rgb(15 23 42 / 0.8);
  }

  :global(.dark) .storage-option.selected {
    border-color: rgb(34 211 238);
    background: rgb(8 47 73 / 0.5);
  }
</style>
