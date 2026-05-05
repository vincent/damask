<script lang="ts">
  import { page } from '$app/state'
  import { ArrowLeft } from '@lucide/svelte'

  const status = $derived(page.status)
  const is404 = $derived(status === 404)
</script>

<svelte:head>
  <title>{status} — Damask</title>
</svelte:head>

<div class="err-root">
  <!-- Damascus geometric tile background -->
  <svg class="err-bg" aria-hidden="true" xmlns="http://www.w3.org/2000/svg">
    <defs>
      <!--
        8-pointed star (khatam) tile — the namesake Damascus pattern.
        72×72 unit cell: outer octagon → inner star traces → corner diamonds.
      -->
      <pattern
        id="khatam"
        x="0"
        y="0"
        width="72"
        height="72"
        patternUnits="userSpaceOnUse"
      >
        <!-- Cell ground -->
        <rect width="72" height="72" fill="transparent" />

        <!-- Outer octagon -->
        <polygon
          points="21,0 51,0 72,21 72,51 51,72 21,72 0,51 0,21"
          fill="none"
          stroke="#b8936a"
          stroke-width="0.6"
          opacity="0.18"
        />

        <!-- 8-pointed star — two overlapping squares rotated 45° -->
        <polygon
          points="36,8 44,28 64,28 50,40 56,60 36,48 16,60 22,40 8,28 28,28"
          fill="none"
          stroke="#b8936a"
          stroke-width="0.5"
          opacity="0.14"
        />
        <polygon
          points="36,14 46,30 62,36 46,42 36,58 26,42 10,36 26,30"
          fill="none"
          stroke="#c9a87c"
          stroke-width="0.4"
          opacity="0.10"
        />

        <!-- Inner star fill hint -->
        <polygon
          points="36,22 40,30 49,30 43,36 45,45 36,40 27,45 29,36 23,30 32,30"
          fill="#b8936a"
          fill-opacity="0.04"
          stroke="#b8936a"
          stroke-width="0.3"
          opacity="0.18"
        />

        <!-- Corner diamond accents at tile intersections -->
        <polygon points="0,0 3,3 0,6 -3,3" fill="#c9a87c" opacity="0.20" />
        <polygon points="72,0 75,3 72,6 69,3" fill="#c9a87c" opacity="0.20" />
        <polygon points="0,72 3,75 0,78 -3,75" fill="#c9a87c" opacity="0.20" />
        <polygon
          points="72,72 75,75 72,78 69,75"
          fill="#c9a87c"
          opacity="0.20"
        />

        <!-- Center medallion dot -->
        <circle cx="36" cy="36" r="1.4" fill="#b8936a" opacity="0.22" />
      </pattern>

      <!-- Vignette to fade edges -->
      <radialGradient id="err-vignette" cx="50%" cy="50%" r="60%">
        <stop offset="0%" stop-color="#0e0b1f" stop-opacity="0" />
        <stop offset="100%" stop-color="#0e0b1f" stop-opacity="0.88" />
      </radialGradient>

      <!-- Soft glow for the center star -->
      <filter id="err-glow" x="-40%" y="-40%" width="180%" height="180%">
        <feGaussianBlur stdDeviation="6" result="b" />
        <feMerge
          ><feMergeNode in="b" /><feMergeNode in="SourceGraphic" /></feMerge
        >
      </filter>
    </defs>

    <!-- Tile fill -->
    <rect width="100%" height="100%" fill="#0e0b1f" />
    <rect width="100%" height="100%" fill="url(#khatam)" />
    <rect width="100%" height="100%" fill="url(#err-vignette)" />

    <!-- Center 8-pointed star medallion, drawn in SVG coordinates -->
    <!-- Positioned at 50% 50% via foreignObject trick — use a centered <g> via CSS transform -->
    <g class="err-star-group">
      <!-- Outer ring -->
      <circle
        r="96"
        fill="none"
        stroke="#b8936a"
        stroke-width="0.7"
        opacity="0.18"
      />
      <circle
        r="72"
        fill="none"
        stroke="#b8936a"
        stroke-width="0.5"
        stroke-dasharray="3 6"
        opacity="0.14"
      />

      <!-- Large 8-pointed star -->
      <polygon
        points="0,-68 18,-18 68,0 18,18 0,68 -18,18 -68,0 -18,-18"
        fill="#13102a"
        fill-opacity="0.95"
        stroke="#b8936a"
        stroke-width="1.2"
        filter="url(#err-glow)"
        opacity="0.55"
      />

      <!-- Inner 8-pointed star (rotated 22.5°) -->
      <polygon
        points="0,-46 12,-12 46,0 12,12 0,46 -12,12 -46,0 -12,-12"
        fill="none"
        stroke="#c9a87c"
        stroke-width="0.6"
        opacity="0.30"
      />

      <!-- Innermost octagon -->
      <polygon
        points="0,-28 20,-20 28,0 20,20 0,28 -20,20 -28,0 -20,-20"
        fill="none"
        stroke="#b8936a"
        stroke-width="0.5"
        opacity="0.22"
      />
    </g>
  </svg>

  <!-- Content -->
  <div class="err-content">
    <!-- Status number — ghosted behind, structural not decorative -->
    <div class="err-status" aria-hidden="true">{status}</div>

    <!-- Diamond divider — Damascus tile corner motif -->
    <div class="err-divider">
      <div class="err-divider-line"></div>
      <svg width="10" height="10" viewBox="-5 -5 10 10" aria-hidden="true">
        <polygon
          points="0,-4.5 4.5,0 0,4.5 -4.5,0"
          fill="#b8936a"
          opacity="0.7"
        />
      </svg>
      <div class="err-divider-line"></div>
    </div>

    {#if is404}
      <h1 class="err-heading">Page not found</h1>
      <p class="err-body">
        This path doesn't exist in your library. It may have been moved or
        deleted.
      </p>
    {:else}
      <h1 class="err-heading">Something went wrong</h1>
      <p class="err-body">
        {page.error?.message ?? 'An unexpected error occurred.'}
      </p>
    {/if}

    <div class="err-actions">
      <a href="/library" class="err-btn-primary">
        <ArrowLeft size={15} />
        Back to library
      </a>
      {#if !is404}
        <button
          onclick={() => window.location.reload()}
          class="err-btn-secondary"
        >
          Reload page
        </button>
      {/if}
    </div>

    <p class="err-footer">HTTP {status} · Damask DAM</p>
  </div>
</div>

<style>
  .err-root {
    position: relative;
    display: flex;
    align-items: center;
    justify-content: center;
    height: 100vh;
    overflow: hidden;
    background: #0e0b1f;
  }

  /* Full-bleed SVG background */
  .err-bg {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    pointer-events: none;
  }

  /* Center the star group at viewport midpoint */
  :global(.err-star-group) {
    transform: translate(50%, 50%);
  }

  /* Content layer */
  .err-content {
    position: relative;
    z-index: 10;
    display: flex;
    flex-direction: column;
    align-items: center;
    text-align: center;
    padding: 0 1.5rem;
  }

  /* Ghosted status number */
  .err-status {
    font-size: clamp(5rem, 14vw, 9rem);
    font-weight: 700;
    letter-spacing: -0.05em;
    line-height: 1;
    color: #b8936a;
    opacity: 0.08;
    user-select: none;
    margin-bottom: -1.5rem; /* overlap below into divider space */
  }

  /* Divider */
  .err-divider {
    display: flex;
    align-items: center;
    gap: 0.625rem;
    margin-bottom: 1.5rem;
  }

  .err-divider-line {
    width: 48px;
    height: 1px;
    background: linear-gradient(to right, transparent, #b8936a55);
  }
  .err-divider-line:last-child {
    background: linear-gradient(to left, transparent, #b8936a55);
  }

  /* Typography */
  .err-heading {
    font-size: 1.0625rem;
    font-weight: 600;
    letter-spacing: -0.01em;
    color: #e8dcc8;
    margin: 0 0 0.5rem;
  }

  .err-body {
    font-size: 0.8125rem;
    line-height: 1.55;
    color: #7a6a55;
    max-width: 28ch;
    margin: 0;
  }

  /* Actions */
  .err-actions {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    margin-top: 2rem;
  }

  .err-btn-primary {
    display: inline-flex;
    align-items: center;
    gap: 0.375rem;
    padding: 0.5rem 1rem;
    border-radius: 6px;
    font-size: 0.8125rem;
    font-weight: 500;
    background: #b8936a;
    color: #0f0c08;
    text-decoration: none;
    transition:
      background 0.15s ease,
      opacity 0.15s ease;
  }
  .err-btn-primary:hover {
    background: #c9a87c;
  }
  .err-btn-primary:focus-visible {
    outline: 2px solid #b8936a;
    outline-offset: 2px;
  }

  .err-btn-secondary {
    display: inline-flex;
    align-items: center;
    gap: 0.375rem;
    padding: 0.5rem 1rem;
    border-radius: 6px;
    font-size: 0.8125rem;
    font-weight: 500;
    background: transparent;
    color: #7a6a55;
    border: 1px solid #2a2240;
    cursor: pointer;
    transition:
      border-color 0.15s ease,
      color 0.15s ease;
  }
  .err-btn-secondary:hover {
    border-color: #b8936a55;
    color: #b8936a;
  }
  .err-btn-secondary:focus-visible {
    outline: 2px solid #b8936a;
    outline-offset: 2px;
  }

  /* Footer */
  .err-footer {
    margin-top: 2.5rem;
    font-family: ui-monospace, 'Cascadia Code', monospace;
    font-size: 0.625rem;
    font-weight: 500;
    letter-spacing: 0.14em;
    text-transform: uppercase;
    color: #2a2240;
  }

  @media (prefers-reduced-motion: no-preference) {
    .err-btn-primary,
    .err-btn-secondary {
      transition:
        background 0.15s ease,
        border-color 0.15s ease,
        color 0.15s ease;
    }
  }
</style>
