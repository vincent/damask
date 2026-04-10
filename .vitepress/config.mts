import { defineConfig } from 'vitepress'
import lightbox from "vitepress-plugin-lightbox"

// https://vitepress.dev/reference/site-config
export default defineConfig({
  srcDir: "docs",
  
  head: [
    [
      'script',
      { async: '', src: 'https://www.googletagmanager.com/gtag/js?id=G-393D2KZD43' }
    ],
    [
      'script',
      {},
      "window.dataLayer = window.dataLayer || [];\nfunction gtag(){dataLayer.push(arguments);}\ngtag('js', new Date());\ngtag('config', 'G-393D2KZD43');"
    ]
  ],

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
  },

  markdown: {
    config: (md) => {
      // Use lightbox plugin
      md.use(lightbox, {});
    },
  },
})

