<script lang="ts">
  import { onMount } from 'svelte'
  import HomepageDiagram from '$lib/components/HomepageDiagram.svelte'

  let scrolled = $state(false)

  onMount(() => {
    const onScroll = () => {
      scrolled = window.scrollY > 60
    }
    window.addEventListener('scroll', onScroll, { passive: true })

    const observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (entry.isIntersecting) {
            entry.target.classList.add('visible')
            // observer.unobserve(entry.target)
          }
        })
      },
      { threshold: 0.1, rootMargin: '0px 0px -60px 0px' }
    )

    // defer so Svelte has flushed the DOM and elements are measurable
    requestAnimationFrame(() => {
      document.querySelectorAll('.reveal').forEach((el) => observer.observe(el))
    })

    return () => {
      window.removeEventListener('scroll', onScroll)
      observer.disconnect()
    }
  })
</script>

<svelte:head>
  <title>Damask: Self-hosted digital asset management</title>
  <meta
    name="description"
    content="Damask is a self-hosted digital asset management system for creative teams that want full control over their files and metadata."
  />
  <link rel="preconnect" href="https://fonts.googleapis.com" />
  <link
    rel="preconnect"
    href="https://fonts.gstatic.com"
    crossorigin="anonymous"
  />
  <link
    href="https://fonts.googleapis.com/css2?family=Bricolage+Grotesque:opsz,wght@12..96,400;12..96,500;12..96,700;12..96,800&display=swap"
    rel="stylesheet"
  />
</svelte:head>

<!-- NAV -->
<header class="hp-nav" class:scrolled>
  <div class="hp-nav-inner">
    <a href="/" class="hp-wordmark" aria-label="Damask home">
      <span class="hp-wordmark-d">d</span>amask
    </a>
    <div class="hp-nav-actions">
      <a
        href="https://docs.damask.studio/"
        class="hp-nav-link"
        target="_blank"
        rel="noopener"
      >
        Docs
      </a>
      <a
        href="https://docs.damask.studio/getting-started"
        class="hp-btn-primary"
        target="_blank"
        rel="noopener"
      >
        Get started
      </a>
    </div>
  </div>
</header>

<!-- HERO -->
<section class="hp-hero">
  <div class="hp-hero-texture" aria-hidden="true"></div>
  <div class="hp-hero-inner">
    <p class="hp-hero-eyebrow reveal">Self-hosted DAM</p>
    <h1 class="hp-hero-headline reveal">
      Your assets.<br />Your server.
    </h1>
    <p class="hp-hero-sub reveal">
      Damask is a self-hosted digital asset management system for creative teams
      that want full control over their files and metadata.
    </p>
    <div class="hp-hero-ctas reveal">
      <a
        href="https://docs.damask.studio/getting-started"
        class="hp-hero-btn-primary"
        target="_blank"
        rel="noopener"
      >
        Get started
      </a>
      <a
        href="https://github.com/vincent/damask"
        class="hp-hero-btn-ghost"
        target="_blank"
        rel="noopener"
      >
        <svg
          width="18"
          height="18"
          viewBox="0 0 24 24"
          fill="currentColor"
          aria-hidden="true"
        >
          <path
            d="M12 2C6.477 2 2 6.477 2 12c0 4.42 2.865 8.17 6.839 9.49.5.092.682-.217.682-.482 0-.237-.008-.866-.013-1.7-2.782.604-3.369-1.34-3.369-1.34-.454-1.156-1.11-1.464-1.11-1.464-.908-.62.069-.608.069-.608 1.003.07 1.531 1.03 1.531 1.03.892 1.529 2.341 1.087 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.11-4.555-4.943 0-1.091.39-1.984 1.029-2.683-.103-.253-.446-1.27.098-2.647 0 0 .84-.269 2.75 1.025A9.578 9.578 0 0112 6.836a9.59 9.59 0 012.504.337c1.909-1.294 2.747-1.025 2.747-1.025.546 1.377.202 2.394.1 2.647.64.699 1.028 1.592 1.028 2.683 0 3.842-2.339 4.687-4.566 4.935.359.309.678.919.678 1.852 0 1.336-.012 2.415-.012 2.743 0 .267.18.578.688.48C19.138 20.167 22 16.418 22 12c0-5.523-4.477-10-10-10z"
          />
        </svg>
        View on GitHub
      </a>
      <a
        href="https://staging.damask.studio/login"
        class="hp-hero-btn-ghost"
        style="border: none;"
        target="_blank"
        rel="noopener"
      >
        or try the demo !
      </a>
    </div>
    <p class="hp-hero-note reveal">
      Runs on your hardware. No usage caps. No per-seat tiers.
    </p>
  </div>
</section>

<!-- PROBLEM -->
<section class="hp-problem">
  <div class="hp-problem-inner reveal">
    <p class="hp-problem-text">
      Cloud DAM tools work until they don't: the pricing jumps, the contract
      ends, or the upload quota runs out. And even when they work, your files
      sit on someone else's server, behind a third-party login, in a format you
      didn't choose.
    </p>
    <p class="hp-problem-answer">
      Damask runs on your machine or your server. Your storage, your rules, your
      backups.
    </p>
  </div>
</section>

<!-- SCREENSHOT -->
<div class="hp-screenshot-wrap">
  <div class="hp-screenshot reveal">
    <img
      src="/screenshot.webp"
      alt="Damask asset library interface"
      class="hp-screenshot-img"
    />
  </div>
</div>

<!-- WHO IT'S FOR -->
<section class="hp-who">
  <div class="hp-who-inner">
    <div class="hp-who-profile reveal" style="--delay: 0ms">
      <span class="hp-who-label">Solo creators</span>
      <h2 class="hp-who-heading">One person,<br />thousands of files.</h2>
      <p class="hp-who-body">
        Damask replaces a folder structure that stopped making sense two years
        ago. Fast upload, instant search, and a clean way to find the right
        version of anything.
      </p>
    </div>
    <div class="hp-who-profile reveal" style="--delay: 80ms">
      <span class="hp-who-label">Creative teams</span>
      <h2 class="hp-who-heading">Shared work,<br />clear boundaries.</h2>
      <p class="hp-who-body">
        Designers, photographers, video editors working on shared projects.
        Roles and permissions keep work organized without locking people out.
        Everyone sees what they need, nothing more.
      </p>
    </div>
    <div class="hp-who-profile reveal" style="--delay: 160ms">
      <span class="hp-who-label">Studios and agencies</span>
      <h2 class="hp-who-heading">Every delivery,<br />every version.</h2>
      <p class="hp-who-body">
        Client deliverables, brand assets, production libraries. Damask tracks
        who uploaded what, when, and what changed. Share work with clients
        without giving them an account. Keep the audit trail in-house.
      </p>
    </div>
  </div>
</section>

<!-- FEATURES -->
<section class="hp-features">
  <div class="hp-features-inner">
    <div class="hp-features-header reveal">
      <h2 class="hp-features-title">Everything a DAM should do.</h2>
      <p class="hp-features-sub">
        Not a simplified version of it. The full thing.
      </p>
    </div>

    <div class="hp-features-grid">
      <div class="hp-feature-group reveal" style="--delay: 0ms">
        <h3 class="hp-feature-group-name">Asset library</h3>
        <ul class="hp-feature-list">
          <li>Drag-and-drop upload, multiple files at once</li>
          <li>Background thumbnail generation</li>
          <li>List and grid views, compact or spaced</li>
          <li>Full-text search across filenames and custom fields</li>
          <li>Filter by type, project, folder, tag, date, or custom field</li>
          <li>EXIF extracted on ingest, GPS stripped by default</li>
        </ul>
      </div>

      <div class="hp-feature-group reveal" style="--delay: 60ms">
        <h3 class="hp-feature-group-name">Organization</h3>
        <ul class="hp-feature-list">
          <li>Projects: group by client, campaign, or shoot</li>
          <li>Folders: two-level depth, drag assets between them</li>
          <li>Tags: workspace-scoped, auto-created on first use</li>
          <li>Collections: cross-project named sets</li>
          <li>Custom fields: text, number, date, single/multi-select</li>
        </ul>
      </div>

      <div class="hp-feature-group reveal" style="--delay: 120ms">
        <h3 class="hp-feature-group-name">Variants and transforms</h3>
        <ul class="hp-feature-list">
          <li>Non-destructive: the original is never touched</li>
          <li>Resize, format conversion</li>
          <li>Manual crop or smart content-aware crop</li>
          <li>Watermark overlay</li>
          <li>Background removal via Remove.bg API</li>
          <li>Video transcoding, frame capture, audio normalization</li>
        </ul>
      </div>

      <div class="hp-feature-group reveal" style="--delay: 0ms">
        <h3 class="hp-feature-group-name">Versioning</h3>
        <ul class="hp-feature-list">
          <li>Upload a new file: previous version is preserved</li>
          <li>One-click rollback with full ownership history</li>
          <li>Share links always point to the current version</li>
        </ul>
      </div>

      <div class="hp-feature-group reveal" style="--delay: 60ms">
        <h3 class="hp-feature-group-name">Sharing</h3>
        <ul class="hp-feature-list">
          <li>No account required for recipients</li>
          <li>Password protection and expiry dates on any share</li>
          <li>Client review mode: view and comment without a Damask account</li>
        </ul>
      </div>

      <div class="hp-feature-group reveal" style="--delay: 120ms">
        <h3 class="hp-feature-group-name">Ingress</h3>
        <ul class="hp-feature-list">
          <li>Email ingest: attach files, route via subaddressing</li>
          <li>SFTP, WebDAV, S3-compatible sources</li>
          <li>Google Drive and Canva OAuth connections</li>
          <li>Pull on a schedule or trigger via webhook</li>
        </ul>
      </div>

      <div class="hp-feature-group reveal" style="--delay: 0ms">
        <h3 class="hp-feature-group-name">Audit log</h3>
        <ul class="hp-feature-list">
          <li>Every metadata change leaves a record</li>
          <li>Renames, retags, moves, uploads, restores, share</li>
          <li>Append-only, configurable retention period</li>
        </ul>
      </div>

      <div class="hp-feature-group reveal" style="--delay: 60ms">
        <h3 class="hp-feature-group-name">Access control</h3>
        <ul class="hp-feature-list">
          <li>Three roles: owner, editor, viewer</li>
          <li>Workspace invites via short-lived token</li>
          <li>OIDC login: Keycloak, Authelia, Authentik, Okta, Auth0</li>
        </ul>
      </div>
    </div>
  </div>
</section>

<!-- DIAGRAM -->
<HomepageDiagram />

<!-- TECH -->
<section class="hp-tech">
  <div class="hp-tech-inner">
    <div class="hp-tech-left reveal">
      <span class="hp-tech-eyebrow">For the technical evaluator</span>
      <h2 class="hp-tech-title">One binary.<br />No other service.</h2>
      <p class="hp-tech-body">
        A single Go binary. Few free external runtime dependencies: FFmpeg,
        ImageMagick. <br />Runs on Linux, macOS, or a cheap VPS. SQLite by
        default : the storage layer is an interface, swap in S3-compatible
        storage in config, no code changes.
      </p>
      <a
        href="https://docs.damask.studio/"
        class="hp-tech-link"
        target="_blank"
        rel="noopener"
      >
        Read the docs
        <svg
          width="14"
          height="14"
          viewBox="0 0 16 16"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
          aria-hidden="true"
        >
          <path d="M3 8h10M9 4l4 4-4 4" />
        </svg>
      </a>
    </div>
    <div class="hp-tech-right reveal" style="--delay: 100ms">
      <ul class="hp-tech-facts">
        <li>
          <span class="hp-tech-fact-name">Backend</span>
          <span class="hp-tech-fact-value">Go + Fiber</span>
        </li>
        <li>
          <span class="hp-tech-fact-name">Frontend</span>
          <span class="hp-tech-fact-value">SvelteKit</span>
        </li>
        <li>
          <span class="hp-tech-fact-name">Database</span>
          <span class="hp-tech-fact-value">SQLite with FTS5</span>
        </li>
        <li>
          <span class="hp-tech-fact-name">Job queue</span>
          <span class="hp-tech-fact-value">In-process, no Redis</span>
        </li>
        <li>
          <span class="hp-tech-fact-name">Multi-tenancy</span>
          <span class="hp-tech-fact-value">One binary, many workspaces</span>
        </li>
        <li>
          <span class="hp-tech-fact-name">Platforms</span>
          <span class="hp-tech-fact-value">Linux, macOS, ARM</span>
        </li>
      </ul>
    </div>
  </div>
</section>

<!-- SELF-HOSTING -->
<section class="hp-selfhost">
  <div class="hp-selfhost-inner">
    <h2 class="hp-selfhost-title reveal">
      Self-hosting, without the ceremony.
    </h2>
    <div class="hp-selfhost-panels">
      <div class="hp-selfhost-panel reveal" style="--delay: 0ms">
        <div class="flex items-center gap-3">
          <span class="hp-selfhost-num">1</span>
          <h3 class="hp-selfhost-panel-title">What you need</h3>
        </div>
        <p class="hp-selfhost-panel-body">
          A Linux server or local machine, Docker or a compiled binary, and a
          storage path.<br />That's it.
        </p>
      </div>
      <div class="hp-selfhost-panel reveal" style="--delay: 80ms">
        <div class="flex items-center gap-3">
          <span class="hp-selfhost-num">2</span>
          <h3 class="hp-selfhost-panel-title">What you keep</h3>
        </div>
        <p class="hp-selfhost-panel-body">
          All files. All metadata. All audit history. No usage caps, no per-seat
          pricing tiers, no data retention clauses.
        </p>
      </div>
      <div class="hp-selfhost-panel reveal" style="--delay: 160ms">
        <div class="flex items-center gap-3">
          <span class="hp-selfhost-num">3</span>
          <h3 class="hp-selfhost-panel-title">When you upgrade</h3>
        </div>
        <p class="hp-selfhost-panel-body">
          Schema migrations run at startup. No manual steps. No downtime window.
          No migration scripts to run by hand.
        </p>
      </div>
    </div>
    <div class="hp-selfhost-ctas reveal">
      <a
        href="https://docs.damask.studio/local-first-remote-optional.html"
        class="hp-selfhost-btn"
        target="_blank"
        rel="noopener"
      >
        Installation docs
      </a>
      <a
        href="https://github.com/vincent/damask/pkgs/container/damask"
        class="hp-selfhost-btn-ghost"
        target="_blank"
        rel="noopener"
      >
        Docker image
      </a>
    </div>
  </div>
</section>

<!-- FOOTER -->
<footer class="hp-footer">
  <div class="hp-footer-inner">
    <a href="/" class="hp-footer-wordmark" aria-label="Damask home">Damask</a>
    <div class="hp-footer-links">
      <a href="https://github.com/vincent/damask" target="_blank" rel="noopener"
        >GitHub</a
      >
      <span class="hp-footer-sep" aria-hidden="true">·</span>
      <span class="hp-footer-license">APL<small>2</small> License</span>
    </div>
  </div>
</footer>

<style>
  /* ─── Tokens ─────────────────────────────────────────────── */
  :root {
    --hp-violet: oklch(40% 0.22 290);
    --hp-violet-light: oklch(46% 0.2 290);
    --hp-violet-pale: oklch(96% 0.04 290);
    --hp-violet-text: oklch(98% 0.01 290);

    --hp-green: oklch(36% 0.16 155);
    --hp-green-mid: oklch(42% 0.18 155);
    --hp-green-pale: oklch(96% 0.04 155);
    --hp-green-text: oklch(98% 0.01 155);

    --hp-cobalt: oklch(38% 0.18 255);
    --hp-cobalt-mid: oklch(44% 0.2 255);
    --hp-cobalt-pale: oklch(96% 0.04 255);
    --hp-cobalt-text: oklch(98% 0.01 255);

    --hp-amber: oklch(68% 0.16 65);
    --hp-amber-dark: oklch(38% 0.1 65);
    --hp-amber-pale: oklch(97% 0.04 65);

    --hp-neutral-bg: oklch(98% 0.008 260);
    --hp-neutral-text: oklch(22% 0.04 260);
    --hp-neutral-muted: oklch(52% 0.04 260);
    --hp-neutral-border: oklch(88% 0.02 260);

    --hp-dark-bg: oklch(16% 0.05 290);
    --hp-dark-surface: oklch(20% 0.06 290);
    --hp-dark-text: oklch(94% 0.02 290);
    --hp-dark-muted: oklch(68% 0.04 290);

    --hp-font-display: var(--font-sans);
    --hp-font-body: ui-sans-serif, system-ui, -apple-system, sans-serif;
  }

  /* ─── Reset for page ─────────────────────────────────────── */
  :global(html) {
    background: var(--hp-dark-bg) !important;
  }

  :global(body:has(.hp-nav)) {
    margin: 0;
    padding: 0;
    overflow-x: hidden;
    background: var(--hp-neutral-bg);
  }

  /* ─── Reveal animation ───────────────────────────────────── */
  .reveal {
    opacity: 0;
    transform: translateY(24px);
    transition:
      opacity 0.55s cubic-bezier(0.16, 1, 0.3, 1),
      transform 0.55s cubic-bezier(0.16, 1, 0.3, 1);
    transition-delay: var(--delay, 0ms);
  }
  :global(.reveal.visible) {
    opacity: 1;
    transform: translateY(0);
  }

  /* ─── Nav ────────────────────────────────────────────────── */
  .hp-nav {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    z-index: 100;
    transition:
      background 0.3s ease,
      border-color 0.3s ease,
      box-shadow 0.3s ease;
    border-bottom: 1px solid transparent;
  }
  .hp-nav.scrolled {
    background: oklch(40% 0.22 290 / 0.96);
    border-color: oklch(50% 0.18 290 / 0.4);
    backdrop-filter: blur(8px);
    -webkit-backdrop-filter: blur(8px);
    box-shadow: 0 1px 12px oklch(10% 0.1 290 / 0.25);
  }
  .hp-nav-inner {
    max-width: 1160px;
    margin: 0 auto;
    padding: 0 clamp(1.25rem, 4vw, 2.5rem);
    height: 62px;
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  .hp-wordmark {
    font-family: var(--hp-font-display);
    font-size: 1.35rem;
    font-weight: 800;
    color: var(--hp-violet-text);
    text-decoration: none;
    letter-spacing: -0.03em;
  }
  .hp-wordmark-d {
    color: oklch(82% 0.12 290);
  }
  .hp-nav-actions {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
  .hp-nav-link {
    font-family: var(--hp-font-body);
    font-size: 0.875rem;
    font-weight: 500;
    color: oklch(88% 0.06 290);
    text-decoration: none;
    padding: 0.375rem 0.75rem;
    border-radius: 6px;
    transition: color 0.15s ease;
  }
  .hp-nav-link:hover {
    color: var(--hp-violet-text);
  }
  .hp-btn-primary {
    font-family: var(--hp-font-body);
    font-size: 0.875rem;
    font-weight: 600;
    color: var(--hp-violet);
    background: var(--hp-violet-text);
    text-decoration: none;
    padding: 0.4rem 1rem;
    border-radius: 6px;
    transition:
      background 0.15s ease,
      opacity 0.15s ease;
  }
  .hp-btn-primary:hover {
    background: oklch(94% 0.03 290);
  }

  /* ─── Hero ───────────────────────────────────────────────── */
  .hp-hero {
    position: relative;
    background: var(--hp-violet);
    min-height: 100svh;
    display: flex;
    align-items: center;
    overflow: hidden;
  }
  .hp-hero-texture {
    position: absolute;
    inset: 0;
    background-image: url(/sidebar-texture.webp);
    background-size: 400px 400px;
    background-repeat: repeat;
    opacity: 0.18;
    pointer-events: none;
  }
  .hp-hero-inner {
    position: relative;
    z-index: 1;
    max-width: 1160px;
    margin: 0 auto;
    padding: clamp(7rem, 12vw, 10rem) clamp(1.25rem, 4vw, 2.5rem)
      clamp(5rem, 8vw, 8rem);
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 0;
  }
  .hp-hero-eyebrow {
    font-family: var(--hp-font-body);
    font-size: 0.8rem;
    font-weight: 600;
    letter-spacing: 0.1em;
    text-transform: uppercase;
    color: oklch(82% 0.1 290);
    margin: 0 0 1.5rem;
  }
  .hp-hero-headline {
    font-family: var(--hp-font-display);
    font-size: clamp(3.5rem, 8vw, 7rem);
    font-weight: 800;
    line-height: 0.95;
    /* letter-spacing: -0.04em; */
    color: var(--hp-violet-text);
    margin: 0 0 clamp(1.25rem, 3vw, 2rem);
  }
  .hp-hero-sub {
    font-family: var(--hp-font-body);
    font-size: clamp(1rem, 2vw, 1.2rem);
    font-weight: 400;
    line-height: 1.65;
    color: oklch(90% 0.05 290);
    max-width: 52ch;
    margin: 0 0 clamp(2rem, 4vw, 3rem);
  }
  .hp-hero-ctas {
    display: flex;
    align-items: center;
    gap: 1rem;
    flex-wrap: wrap;
    margin-bottom: 1.5rem;
  }
  .hp-hero-btn-primary {
    display: inline-flex;
    align-items: center;
    font-family: var(--hp-font-body);
    font-size: 1rem;
    font-weight: 600;
    color: var(--hp-violet);
    background: var(--hp-violet-text);
    text-decoration: none;
    padding: 0.75rem 1.75rem;
    border-radius: 8px;
    transition:
      background 0.15s ease,
      transform 0.15s ease;
  }
  .hp-hero-btn-primary:hover {
    background: oklch(94% 0.03 290);
    transform: translateY(-1px);
  }
  .hp-hero-btn-ghost {
    display: inline-flex;
    align-items: center;
    gap: 0.5rem;
    font-family: var(--hp-font-body);
    font-size: 0.9375rem;
    font-weight: 500;
    color: oklch(88% 0.06 290);
    text-decoration: none;
    padding: 0.75rem 1.5rem;
    border-radius: 8px;
    border: 1px solid oklch(60% 0.12 290 / 0.5);
    transition:
      color 0.15s ease,
      border-color 0.15s ease,
      background 0.15s ease;
  }
  .hp-hero-btn-ghost:hover {
    color: var(--hp-violet-text);
    border-color: oklch(70% 0.1 290 / 0.7);
    background: oklch(100% 0 0 / 0.06);
  }
  .hp-hero-note {
    font-family: var(--hp-font-body);
    font-size: 0.8125rem;
    color: oklch(76% 0.08 290);
    margin: 0;
  }

  /* ─── Problem ────────────────────────────────────────────── */
  .hp-problem {
    background: var(--hp-neutral-bg);
    padding: clamp(4rem, 8vw, 7rem) clamp(1.25rem, 4vw, 2.5rem);
  }
  .hp-problem-inner {
    max-width: 640px;
    margin: 0 auto;
    text-align: center;
  }
  .hp-problem-text {
    font-family: var(--hp-font-body);
    font-size: clamp(1rem, 1.8vw, 1.175rem);
    line-height: 1.75;
    color: var(--hp-neutral-muted);
    margin: 0 0 1.75rem;
  }
  .hp-problem-answer {
    font-family: var(--hp-font-display);
    font-size: clamp(1.2rem, 2.2vw, 1.5rem);
    font-weight: 700;
    line-height: 1.4;
    color: var(--hp-neutral-text);
    margin: 0;
  }

  /* ─── Screenshot ────────────────────────────────────────── */
  .hp-screenshot-wrap {
    background: var(--hp-neutral-bg);
    padding: 0 clamp(1.25rem, 4vw, 2.5rem);
    display: flex;
    justify-content: center;
    padding-bottom: 2em;
  }
  .hp-screenshot {
    max-width: 1160px;
    width: 100%;
    border-radius: 14px;
    overflow: hidden;
    box-shadow:
      0 2px 4px oklch(20% 0.08 260 / 0.06),
      0 8px 24px oklch(20% 0.08 260 / 0.1),
      0 32px 64px oklch(20% 0.08 260 / 0.12);
    border: 1px solid var(--hp-dark-bg);
    transform: translateY(3rem);
    margin-bottom: 3rem;
  }
  .hp-screenshot-img {
    display: block;
    width: 100%;
    height: auto;
  }

  /* ─── Who it's for ───────────────────────────────────────── */
  .hp-who {
    background: var(--hp-green);
    padding: clamp(4rem, 8vw, 7rem) clamp(1.25rem, 4vw, 2.5rem);
    position: relative;
    overflow: hidden;
  }
  .hp-who-inner {
    max-width: 1160px;
    margin: 0 auto;
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
    gap: clamp(2rem, 4vw, 3.5rem);
  }
  .hp-who-profile {
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }
  .hp-who-label {
    font-family: var(--hp-font-body);
    font-size: 0.75rem;
    font-weight: 600;
    letter-spacing: 0.1em;
    text-transform: uppercase;
    color: oklch(76% 0.1 155);
  }
  .hp-who-heading {
    font-family: var(--hp-font-display);
    font-size: clamp(1.6rem, 3vw, 2.2rem);
    font-weight: 800;
    line-height: 1.1;
    letter-spacing: -0.03em;
    color: oklch(98% 0.01 155);
    margin: 0;
  }
  .hp-who-body {
    font-family: var(--hp-font-body);
    font-size: 0.9375rem;
    line-height: 1.7;
    color: oklch(88% 0.05 155);
    margin: 0;
    max-width: 42ch;
  }

  /* ─── Features ───────────────────────────────────────────── */
  .hp-features {
    background: var(--hp-cobalt);
    padding: clamp(4rem, 8vw, 7rem) clamp(1.25rem, 4vw, 2.5rem);
  }
  .hp-features-inner {
    max-width: 1160px;
    margin: 0 auto;
  }
  .hp-features-header {
    margin-bottom: clamp(2.5rem, 5vw, 4.5rem);
  }
  .hp-features-title {
    font-family: var(--hp-font-display);
    font-size: clamp(2rem, 4vw, 3.2rem);
    font-weight: 800;
    line-height: 1.1;
    letter-spacing: -0.035em;
    color: oklch(98% 0.01 255);
    margin: 0 0 0.75rem;
  }
  .hp-features-sub {
    font-family: var(--hp-font-body);
    font-size: 1rem;
    color: oklch(80% 0.06 255);
    margin: 0;
  }
  .hp-features-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
    gap: clamp(2rem, 3.5vw, 3rem);
  }
  .hp-feature-group {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
  }
  .hp-feature-group-name {
    font-family: var(--hp-font-body);
    font-size: 0.75rem;
    font-weight: 700;
    letter-spacing: 0.09em;
    text-transform: uppercase;
    color: oklch(75% 0.1 255);
    margin: 0 0 0.25rem;
    padding-bottom: 0.625rem;
    border-bottom: 1px solid oklch(55% 0.12 255 / 0.35);
  }
  .hp-feature-list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }
  .hp-feature-list li {
    font-family: var(--hp-font-body);
    font-size: 0.875rem;
    line-height: 1.55;
    color: oklch(88% 0.04 255);
    padding-left: 1.1rem;
    position: relative;
  }
  .hp-feature-list li::before {
    content: '—';
    position: absolute;
    left: 0;
    color: oklch(65% 0.1 255);
    font-size: 0.75rem;
    top: 0.08em;
  }

  /* ─── Tech ───────────────────────────────────────────────── */
  .hp-tech {
    background: var(--hp-dark-bg);
    padding: clamp(4rem, 8vw, 7rem) clamp(1.25rem, 4vw, 2.5rem);
  }
  .hp-tech-inner {
    max-width: 1160px;
    margin: 0 auto;
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: clamp(3rem, 6vw, 6rem);
    align-items: center;
  }
  .hp-tech-eyebrow {
    display: block;
    font-family: var(--hp-font-body);
    font-size: 0.75rem;
    font-weight: 600;
    letter-spacing: 0.1em;
    text-transform: uppercase;
    color: oklch(62% 0.06 290);
    margin-bottom: 1rem;
  }
  .hp-tech-title {
    font-family: var(--hp-font-display);
    font-size: clamp(2rem, 3.5vw, 3rem);
    font-weight: 800;
    line-height: 1.1;
    letter-spacing: -0.035em;
    color: var(--hp-dark-text);
    margin: 0 0 1.25rem;
  }
  .hp-tech-body {
    font-family: var(--hp-font-body);
    font-size: 0.9375rem;
    line-height: 1.75;
    color: var(--hp-dark-muted);
    margin: 0 0 2rem;
    max-width: 48ch;
  }
  .hp-tech-link {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    font-family: var(--hp-font-body);
    font-size: 0.875rem;
    font-weight: 600;
    color: oklch(78% 0.1 290);
    text-decoration: none;
    transition:
      color 0.15s ease,
      gap 0.15s ease;
  }
  .hp-tech-link:hover {
    color: var(--hp-dark-text);
    gap: 0.6rem;
  }
  .hp-tech-facts {
    list-style: none;
    margin: 0;
    padding: 0;
    background: var(--hp-dark-surface);
    border-radius: 12px;
    border: 1px solid oklch(28% 0.06 290);
    overflow: hidden;
  }
  .hp-tech-facts li {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    padding: 0.875rem 1.25rem;
    border-bottom: 1px solid oklch(24% 0.05 290);
  }
  .hp-tech-facts li:last-child {
    border-bottom: none;
  }
  .hp-tech-fact-name {
    font-family: var(--hp-font-body);
    font-size: 0.8125rem;
    font-weight: 500;
    color: var(--hp-dark-muted);
  }
  .hp-tech-fact-value {
    font-family: ui-monospace, 'Cascadia Code', 'Source Code Pro', monospace;
    font-size: 0.8125rem;
    color: oklch(78% 0.08 290);
  }

  /* ─── Self-hosting ───────────────────────────────────────── */
  .hp-selfhost {
    background: var(--hp-amber-pale);
    padding: clamp(4rem, 8vw, 7rem) clamp(1.25rem, 4vw, 2.5rem);
  }
  .hp-selfhost-inner {
    max-width: 1160px;
    margin: 0 auto;
  }
  .hp-selfhost-title {
    font-family: var(--hp-font-display);
    font-size: clamp(2rem, 4vw, 3rem);
    font-weight: 800;
    line-height: 1.1;
    letter-spacing: -0.035em;
    color: var(--hp-amber-dark);
    margin: 0 0 clamp(2.5rem, 5vw, 4rem);
  }
  .hp-selfhost-panels {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
    gap: clamp(1.5rem, 3vw, 2.5rem);
    margin-bottom: clamp(2.5rem, 5vw, 4rem);
  }
  .hp-selfhost-panel {
    background: oklch(99% 0.02 65);
    border: 1px solid oklch(88% 0.08 65);
    border-radius: 10px;
    padding: clamp(1.5rem, 2.5vw, 2rem);
    display: flex;
    flex-direction: column;
    gap: 0.875rem;
  }
  .hp-selfhost-num {
    font-family: var(--hp-font-display);
    font-size: 2.5rem;
    font-weight: 800;
    line-height: 1;
    color: oklch(82% 0.12 65);
    /* letter-spacing: -0.04em; */
  }
  .hp-selfhost-panel-title {
    font-family: var(--hp-font-display);
    font-size: 1.1rem;
    font-weight: 700;
    color: var(--hp-amber-dark);
    margin: 0;
  }
  .hp-selfhost-panel-body {
    font-family: var(--hp-font-body);
    font-size: 0.9375rem;
    line-height: 1.7;
    color: oklch(42% 0.08 65);
    margin: 0;
  }
  .hp-selfhost-ctas {
    display: flex;
    align-items: center;
    gap: 1rem;
    flex-wrap: wrap;
  }
  .hp-selfhost-btn {
    display: inline-flex;
    align-items: center;
    font-family: var(--hp-font-body);
    font-size: 0.9375rem;
    font-weight: 600;
    color: var(--hp-amber-pale);
    background: var(--hp-amber-dark);
    text-decoration: none;
    padding: 0.7rem 1.5rem;
    border-radius: 8px;
    transition:
      background 0.15s ease,
      transform 0.15s ease;
  }
  .hp-selfhost-btn:hover {
    background: oklch(30% 0.09 65);
    transform: translateY(-1px);
  }
  .hp-selfhost-btn-ghost {
    display: inline-flex;
    align-items: center;
    font-family: var(--hp-font-body);
    font-size: 0.9375rem;
    font-weight: 500;
    color: var(--hp-amber-dark);
    text-decoration: none;
    padding: 0.7rem 1.5rem;
    border-radius: 8px;
    border: 1px solid oklch(72% 0.1 65);
    transition:
      background 0.15s ease,
      border-color 0.15s ease;
  }
  .hp-selfhost-btn-ghost:hover {
    background: oklch(94% 0.05 65);
    border-color: oklch(62% 0.12 65);
  }

  /* ─── Footer ─────────────────────────────────────────────── */
  .hp-footer {
    background: var(--hp-dark-bg);
    border-top: 1px solid oklch(24% 0.05 290);
    padding: 1.75rem clamp(1.25rem, 4vw, 2.5rem);
  }
  .hp-footer-inner {
    max-width: 1160px;
    margin: 0 auto;
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  .hp-footer-wordmark {
    font-family: var(--hp-font-display);
    font-size: 1rem;
    font-weight: 800;
    letter-spacing: -0.03em;
    color: var(--hp-dark-muted);
    text-decoration: none;
    transition: color 0.15s ease;
  }
  .hp-footer-wordmark:hover {
    color: var(--hp-dark-text);
  }
  .hp-footer-links {
    display: flex;
    align-items: center;
    gap: 0.625rem;
    font-family: var(--hp-font-body);
    font-size: 0.8125rem;
    color: var(--hp-dark-muted);
  }
  .hp-footer-links a {
    color: var(--hp-dark-muted);
    text-decoration: none;
    transition: color 0.15s ease;
  }
  .hp-footer-links a:hover {
    color: var(--hp-dark-text);
  }
  .hp-footer-sep {
    opacity: 0.4;
  }

  /* ─── Responsive ─────────────────────────────────────────── */
  @media (max-width: 768px) {
    .hp-tech-inner {
      grid-template-columns: 1fr;
    }
    .hp-tech-body {
      max-width: none;
    }
    .hp-hero-headline {
      font-size: clamp(3rem, 12vw, 4.5rem);
    }
    .hp-who-body {
      max-width: none;
    }
  }

  @media (max-width: 480px) {
    .hp-hero-ctas {
      flex-direction: column;
      align-items: flex-start;
    }
    .hp-hero-btn-primary,
    .hp-hero-btn-ghost {
      width: 100%;
      justify-content: center;
    }
  }

  /* ─── Reduced motion ─────────────────────────────────────── */
  @media (prefers-reduced-motion: reduce) {
    .reveal {
      transition: none;
      opacity: 1;
      transform: none;
    }
  }
</style>
