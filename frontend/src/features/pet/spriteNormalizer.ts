import type { PetAsset } from '@/api/pet'

const CELL_WIDTH = 192
const CELL_HEIGHT = 208
const ROW_FRAME_COUNTS = [6, 8, 8, 4, 5, 8, 6, 6, 6, 8, 8] as const

export interface PetFrameBounds {
  left: number
  top: number
  right: number
  bottom: number
}

export interface PetRowMetric {
  row: number
  medianHeight: number
  anchorX: number
  baseline: number
  frames: PetFrameBounds[]
}

export interface PetRowTransform {
  row: number
  scale: number
  anchorX: number
  baseline: number
}

function median(values: number[]): number {
  if (values.length === 0) return 0
  const sorted = [...values].sort((a, b) => a - b)
  const middle = Math.floor(sorted.length / 2)
  return sorted.length % 2 === 0 ? (sorted[middle - 1] + sorted[middle]) / 2 : sorted[middle]
}

export function buildPetRowTransforms(metrics: PetRowMetric[]): PetRowTransform[] {
  const targetHeight = median(metrics.map((metric) => metric.medianHeight).filter((height) => height > 0))
  return metrics.map((metric) => {
    let scale = metric.medianHeight > 0 ? targetHeight / metric.medianHeight : 1
    // Character scale varies substantially between action rows in many Codex
    // pet sheets. Keep one transform for the whole row, but allow enough range
    // to make running, waiting, and review actions read as the same character.
    scale = Math.min(1.6, Math.max(0.65, scale))
    if (scale > 1) {
      for (const frame of metric.frames) {
        const horizontalLimit = Math.min(
          frame.left < metric.anchorX ? (metric.anchorX - 3) / (metric.anchorX - frame.left) : Number.POSITIVE_INFINITY,
          frame.right > metric.anchorX ? (CELL_WIDTH - 3 - metric.anchorX) / (frame.right - metric.anchorX) : Number.POSITIVE_INFINITY,
        )
        const verticalLimit = Math.min(
          frame.top < metric.baseline ? (metric.baseline - 3) / (metric.baseline - frame.top) : Number.POSITIVE_INFINITY,
          frame.bottom > metric.baseline ? (CELL_HEIGHT - 3 - metric.baseline) / (frame.bottom - metric.baseline) : Number.POSITIVE_INFINITY,
        )
        scale = Math.min(scale, horizontalLimit, verticalLimit)
      }
    }
    if (!Number.isFinite(scale) || Math.abs(scale - 1) < 0.05) scale = 1
    return { row: metric.row, scale, anchorX: metric.anchorX, baseline: metric.baseline }
  })
}

export async function normalizePetSpritesheet(blob: Blob, asset: PetAsset): Promise<Blob> {
  if (typeof document === 'undefined' || asset.width !== 1536 || ![1872, 2288].includes(asset.height)) return blob
  const image = await decodeBlob(blob)
  try {
    const source = document.createElement('canvas')
    source.width = asset.width
    source.height = asset.height
    const sourceContext = source.getContext('2d', { willReadFrequently: true })
    if (!sourceContext) return blob
    sourceContext.drawImage(image, 0, 0, asset.width, asset.height)
    const pixels = sourceContext.getImageData(0, 0, asset.width, asset.height).data
    const rowCount = asset.sprite_version === 2 ? 11 : 9
    const metrics: PetRowMetric[] = []
    for (let row = 0; row < rowCount; row += 1) {
      const frames: PetFrameBounds[] = []
      for (let column = 0; column < ROW_FRAME_COUNTS[row]; column += 1) {
        const bounds = alphaBounds(pixels, asset.width, column * CELL_WIDTH, row * CELL_HEIGHT)
        if (bounds) frames.push(bounds)
      }
      if (frames.length === 0) continue
      metrics.push({
        row,
        medianHeight: median(frames.map((frame) => frame.bottom - frame.top + 1)),
        anchorX: median(frames.map((frame) => (frame.left + frame.right) / 2)),
        baseline: median(frames.map((frame) => frame.bottom)),
        frames,
      })
    }
    const transforms = buildPetRowTransforms(metrics)
    if (!transforms.some((transform) => transform.scale !== 1)) return blob

    const output = document.createElement('canvas')
    output.width = asset.width
    output.height = asset.height
    const outputContext = output.getContext('2d')
    if (!outputContext) return blob
    outputContext.imageSmoothingEnabled = true
    outputContext.imageSmoothingQuality = 'high'
    for (const transform of transforms) {
      const frameCount = ROW_FRAME_COUNTS[transform.row]
      for (let column = 0; column < frameCount; column += 1) {
        const cellX = column * CELL_WIDTH
        const cellY = transform.row * CELL_HEIGHT
        outputContext.save()
        outputContext.beginPath()
        outputContext.rect(cellX, cellY, CELL_WIDTH, CELL_HEIGHT)
        outputContext.clip()
        outputContext.drawImage(
          source,
          cellX,
          cellY,
          CELL_WIDTH,
          CELL_HEIGHT,
          cellX + transform.anchorX * (1 - transform.scale),
          cellY + transform.baseline * (1 - transform.scale),
          CELL_WIDTH * transform.scale,
          CELL_HEIGHT * transform.scale,
        )
        outputContext.restore()
      }
    }
    return await canvasBlob(output) || blob
  } finally {
    if ('close' in image && typeof image.close === 'function') image.close()
  }
}

function alphaBounds(
  pixels: Uint8ClampedArray,
  imageWidth: number,
  startX: number,
  startY: number,
): PetFrameBounds | null {
  let left = CELL_WIDTH
  let top = CELL_HEIGHT
  let right = -1
  let bottom = -1
  for (let y = 0; y < CELL_HEIGHT; y += 1) {
    for (let x = 0; x < CELL_WIDTH; x += 1) {
      const alpha = pixels[((startY + y) * imageWidth + startX + x) * 4 + 3]
      if (alpha < 16) continue
      left = Math.min(left, x)
      top = Math.min(top, y)
      right = Math.max(right, x)
      bottom = Math.max(bottom, y)
    }
  }
  return right < left || bottom < top ? null : { left, top, right, bottom }
}

async function decodeBlob(blob: Blob): Promise<ImageBitmap | HTMLImageElement> {
  if (typeof createImageBitmap === 'function') return createImageBitmap(blob)
  const url = URL.createObjectURL(blob)
  try {
    const image = new Image()
    image.decoding = 'async'
    image.src = url
    await image.decode()
    return image
  } finally {
    URL.revokeObjectURL(url)
  }
}

function canvasBlob(canvas: HTMLCanvasElement): Promise<Blob | null> {
  return new Promise((resolve) => canvas.toBlob(resolve, 'image/webp', 0.96))
}
