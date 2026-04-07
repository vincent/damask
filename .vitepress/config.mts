import { defineConfig } from 'vitepress'

// https://vitepress.dev/reference/site-config
export default defineConfig({
  srcDir: "docs",
  
  title: "Damask",
  description: "Damask is a digital asset manager for designers, photographers, and creative studios.",
  themeConfig: {
    // https://vitepress.dev/reference/default-theme-config
    nav: [
      { text: 'Home', link: '/' },
      { text: 'Demo', link: 'https://damask.studio/login'  },
    ],

    sidebar: [
      {
        text: 'Features',
        items: [
          { text: 'Projects, Folders & Tags',     link: '/projects-folders-tags' },
          { text: 'Custom Metadata Fields',       link: '/custom-metadata-fields' },
          { text: 'Version History & Audit Log',  link: '/version-history-audit-log' },
          { text: 'Transforms & Variants',        link: '/transforms-variants' },
          { text: 'Client Delivery & Sharing',    link: '/client-delivery-sharing' },
          { text: 'Automatic Ingestion',          link: '/automatic-ingestion' },
          { text: 'Local-First, Remote-Optional', link: '/local-first-remote-optional' },
          { text: 'Open Source & Self-Hostable',  link: '/open-source-self-hostable' },
          { text: 'Multi-Workspace & Teams',      link: '/multi-workspace-team' },
        ]
      }
    ],

    socialLinks: [
      { icon: 'github', link: 'https://github.com/vincent/damask' }
    ]
  }
})

