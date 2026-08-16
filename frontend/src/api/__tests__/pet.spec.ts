import { beforeEach, describe, expect, it, vi } from 'vitest'

const client = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
  delete: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: client,
  buildApiUrl: (path: string) => `/api/v1${path}`,
}))

import { petAPI, type PetAsset } from '@/api/pet'

const asset: PetAsset = {
  id: 'pet-id',
  pet_key: 'test-pet',
  display_name: 'Test pet',
  description: '',
  sprite_version: 1,
  sha256: 'a'.repeat(64),
  size_bytes: 1,
  width: 1536,
  height: 1872,
  license: '',
  created_at: '2026-08-15T00:00:00Z',
  asset_url: '/api/v1/pet/assets/pet-id/spritesheet',
  is_builtin: true,
}

describe('pet api', () => {
  beforeEach(() => {
    Object.values(client).forEach((mock) => mock.mockReset())
  })

  it('does not duplicate the API prefix when fetching a spritesheet', async () => {
    const blob = new Blob(['sprite'], { type: 'image/webp' })
    client.get.mockResolvedValue({ data: blob })

    await expect(petAPI.fetchSpritesheet(asset)).resolves.toBe(blob)

    expect(client.get).toHaveBeenCalledWith('/pet/assets/pet-id/spritesheet', {
      responseType: 'blob',
      timeout: 60_000,
    })
  })
})
