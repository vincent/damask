export type ShortcutAction =
  | 'palette.open'
  | 'search.focus'
  | 'upload.open'
  | 'asset.delete'
  | 'asset.download'
  | 'asset.rename'
  | 'asset.share'
  | 'asset.open-detail'
  | 'selection.all'
  | 'selection.clear'
  | 'selection.invert'
  | 'view.toggle-layout'
  | 'view.zoom-in'
  | 'view.zoom-out'
  | 'view.zoom-reset'
  | 'lightbox.close'
  | 'lightbox.next'
  | 'lightbox.prev'
  | 'lightbox.download'
  | 'lightbox.zoom-in'
  | 'lightbox.zoom-out'
  | 'navigate.library'
  | 'navigate.projects'
  | 'navigate.tags'
  | 'navigate.settings'
  | 'navigate.shares'
  | 'help.toggle'
  | 'sidebar.toggle';

export type KeyMap = Record<ShortcutAction, string[]>;

export type ShortcutHandler = (action: ShortcutAction) => boolean | void;

export interface HandlerRegistration {
  action: ShortcutAction;
  handler: ShortcutHandler;
  label: string;
}

export interface ShortcutGroup {
  title: string;
  actions: Array<{
    action: ShortcutAction;
    label: string;
    contextual?: boolean;
  }>;
}
