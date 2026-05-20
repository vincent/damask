import type { Asset } from '$lib/api'
import { fireEvent, render, screen } from '@testing-library/svelte'
import { beforeAll, describe, expect, it, vi } from 'vitest'
import VariantToolPanel from './VariantToolPanel.svelte'
import type { VariantTab } from './VariantsTool.svelte'

const asset: Asset = {
  id: 'asset-1',
  workspace_id: 'workspace-1',
  project_id: null,
  folder_id: null,
  derived_from_asset_id: null,
  original_filename: 'image.jpg',
  mime_type: 'image/jpeg',
  size: 1024,
  width: 1200,
  height: 800,
  thumbnail_key: null,
  thumbnail_content_type: null,
  metadata: null,
  tags: [],
  version_count: 1,
  variant_count: 0,
  variants_rebuilding: false,
  created_at: '2026-05-19T00:00:00Z',
  updated_at: '2026-05-19T00:00:00Z',
  created_by: null,
  authors: [],
}

beforeAll(() => {
  const elementPrototype = Element.prototype as unknown as {
    animate?: typeof Element.prototype.animate
  }
  if (elementPrototype.animate) return
  elementPrototype.animate = vi.fn(() => {
    const animation = {
      cancel: vi.fn(),
      commitStyles: vi.fn(),
      finished: Promise.resolve(),
      onfinish: null as ((event: AnimationPlaybackEvent) => void) | null,
      play: vi.fn(),
    }
    setTimeout(() => animation.onfinish?.({} as AnimationPlaybackEvent), 0)
    return animation as unknown as Animation
  })
})

function renderPanel(
  props: Partial<{
    tool: VariantTab
    creating: boolean
    handleCreate: (type: string, params: object) => void
    onClose: () => void
  }> = {}
) {
  return render(VariantToolPanel, {
    tool: 'resize',
    asset,
    creating: false,
    handleCreate: vi.fn(),
    onClose: vi.fn(),
    ...props,
  })
}

describe('VariantToolPanel', () => {
  it('renders the tool label in the header', () => {
    renderPanel({ tool: 'resize' })
    expect(screen.getByRole('heading', { name: 'Resize' })).toBeInTheDocument()
  })

  it('renders the tool subtitle in the header', () => {
    renderPanel({ tool: 'resize' })
    expect(screen.getByText('Change dimensions')).toBeInTheDocument()
  })

  it('renders a close button', () => {
    renderPanel()
    expect(
      screen.getByRole('button', { name: 'Close tool panel' })
    ).toBeInTheDocument()
  })

  it('renders the VariantsTool component', () => {
    renderPanel({ tool: 'resize' })
    expect(screen.getByLabelText('Width (px)')).toBeInTheDocument()
  })

  it('close button click fires onClose', async () => {
    const onClose = vi.fn()
    renderPanel({ onClose })
    await fireEvent.click(
      screen.getByRole('button', { name: 'Close tool panel' })
    )
    expect(onClose).toHaveBeenCalled()
  })

  it('close button is disabled while creating', () => {
    renderPanel({ creating: true })
    expect(
      screen.getByRole('button', { name: 'Close tool panel' })
    ).toBeDisabled()
  })
})
