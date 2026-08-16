import { describe, expect, it } from 'vitest'
import { buildPetRowTransforms, type PetRowMetric } from '../spriteNormalizer'

function metric(row: number, height: number): PetRowMetric {
  const bottom = 174
  return {
    row,
    medianHeight: height,
    anchorX: 96,
    baseline: bottom,
    frames: [{ left: 80, top: bottom - height + 1, right: 112, bottom }],
  }
}

describe('pet sprite normalization', () => {
  it('corrects large cross-action scale differences without per-frame scaling', () => {
    const transforms = buildPetRowTransforms([
      metric(3, 50),
      metric(0, 100),
      metric(6, 150),
    ])

    expect(transforms.find((item) => item.row === 3)?.scale).toBe(1.6)
    expect(transforms.find((item) => item.row === 0)?.scale).toBe(1)
    expect(transforms.find((item) => item.row === 6)?.scale).toBeCloseTo(2 / 3, 4)
    expect(transforms).toEqual(expect.arrayContaining([
      expect.objectContaining({ anchorX: 96, baseline: 174 }),
    ]))
  })
})
