import type { KeyMap, ShortcutGroup } from './types';

export const DEFAULT_KEYMAP: KeyMap = {
  'palette.open':          ['$mod+k'],
  'search.focus':          ['f', 'F'],
  'upload.open':           ['$mod+u'],
  'asset.delete':          ['$mod+Backspace'],
  'asset.download':        ['$mod+d'],
  'asset.rename':          ['F2'],
  'asset.share':           ['$mod+Shift+s'],
  'asset.open-detail':     ['Enter'],
  'selection.all':         ['$mod+a'],
  'selection.clear':       ['Escape'],
  'selection.invert':      ['$mod+Shift+i'],
  'view.toggle-layout':    ['v'],
  'view.zoom-in':          ['$mod+=', '$mod+NumpadAdd', '+'],
  'view.zoom-out':         ['$mod+-', '$mod+NumpadSubtract', '-'],
  'view.zoom-reset':       ['$mod+0'],
  'lightbox.close':        ['Escape'],
  'lightbox.next':         ['ArrowRight', 'l'],
  'lightbox.prev':         ['ArrowLeft', 'h'],
  'lightbox.download':     ['$mod+d'],
  'lightbox.zoom-in':      ['+', '='],
  'lightbox.zoom-out':     ['-'],
  'navigate.library':      ['g l'],
  'navigate.projects':     ['g p'],
  'navigate.tags':         ['g t'],
  'navigate.settings':     ['g s'],
  'navigate.shares':       ['g h'],
  'help.toggle':           ['?', 'Shift+/'],
  'sidebar.toggle':        ['Shift+Tab'],
};

export const SHORTCUT_GROUPS: ShortcutGroup[] = [
  {
    title: 'Global',
    actions: [
      { action: 'palette.open',      label: 'Open command palette' },
      { action: 'search.focus',      label: 'Focus search' },
      { action: 'upload.open',       label: 'Upload assets' },
      { action: 'sidebar.toggle',    label: 'Toggle sidebar' },
      { action: 'help.toggle',       label: 'Show shortcuts' },
    ],
  },
  {
    title: 'Assets',
    actions: [
      { action: 'asset.open-detail', label: 'Open detail panel' },
      { action: 'asset.download',    label: 'Download' },
      { action: 'asset.rename',      label: 'Rename' },
      { action: 'asset.share',       label: 'Share' },
      { action: 'asset.delete',      label: 'Delete' },
      { action: 'selection.all',     label: 'Select all' },
      { action: 'selection.invert',  label: 'Invert selection' },
      { action: 'selection.clear',   label: 'Clear selection' },
    ],
  },
  {
    title: 'View',
    actions: [
      { action: 'view.toggle-layout', label: 'Toggle grid / list' },
      { action: 'view.zoom-in',       label: 'Zoom in' },
      { action: 'view.zoom-out',      label: 'Zoom out' },
      { action: 'view.zoom-reset',    label: 'Reset zoom' },
    ],
  },
  {
    title: 'Lightbox',
    actions: [
      { action: 'lightbox.next',     label: 'Next asset',     contextual: true },
      { action: 'lightbox.prev',     label: 'Previous asset', contextual: true },
      { action: 'lightbox.download', label: 'Download',       contextual: true },
      { action: 'lightbox.zoom-in',  label: 'Zoom in',        contextual: true },
      { action: 'lightbox.zoom-out', label: 'Zoom out',       contextual: true },
      { action: 'lightbox.close',    label: 'Close',          contextual: true },
    ],
  },
  {
    title: 'Navigate',
    actions: [
      { action: 'navigate.library',  label: 'Go to Library' },
      { action: 'navigate.projects', label: 'Go to Projects' },
      { action: 'navigate.tags',     label: 'Go to Tags' },
      { action: 'navigate.shares',   label: 'Go to Shares' },
      { action: 'navigate.settings', label: 'Go to Settings' },
    ],
  },
];
