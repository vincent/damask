<script lang="ts">
  interface Props {
    source: string
  }

  let { source }: Props = $props()

  interface Heading {
    level: 2 | 3
    text: string
    anchor: string
  }

  const headings = $derived(
    [...source.matchAll(/^(#{2,3})\s+(.+)$/gm)].map((m) => ({
      level: m[1].length as 2 | 3,
      text: m[2].trim(),
      anchor: m[2]
        .trim()
        .toLowerCase()
        .replace(/[^\w]+/g, '-'),
    }))
  )
</script>

{#if headings.length > 0}
  <nav class="docs-toc" aria-label="On this page">
    <p class="docs-toc-heading">On this page</p>
    <ul class="docs-toc-list">
      {#each headings as h}
        <li class="docs-toc-item" class:docs-toc-h3={h.level === 3}>
          <a class="docs-toc-link" href="#{h.anchor}">{h.text}</a>
        </li>
      {/each}
    </ul>
  </nav>
{/if}

<style>
  .docs-toc {
    position: sticky;
    top: 1rem;
  }

  .docs-toc-heading {
    font-size: 0.6875rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--text-muted);
    margin: 0 0 0.5rem;
  }

  .docs-toc-list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .docs-toc-item {
    display: block;
  }

  .docs-toc-h3 .docs-toc-link {
    padding-left: 0.75rem;
    font-size: 0.75rem;
  }

  .docs-toc-link {
    display: block;
    font-size: 0.8125rem;
    color: var(--text-secondary);
    text-decoration: none;
    padding: 0.1875rem 0;
    line-height: 1.4;
  }

  .docs-toc-link:hover {
    color: var(--accent-cta);
  }
</style>
