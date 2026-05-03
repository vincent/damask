<script lang="ts">
  import Hint from './Hint.svelte'
  import Title from './Title.svelte'

  let { title, description = undefined, children = undefined } = $props()
</script>

<!-- Header -->
<header
  class="border-b border-[var(--border-subtle)] bg-[var(--bg-surface)] px-6 py-4"
>
  <div class="flex items-center justify-between">
    <div>
      <Title>{title}</Title>
      {#if description}
        <Hint>{description}</Hint>
      {/if}
    </div>
    {@render children?.()}
  </div>
</header>
