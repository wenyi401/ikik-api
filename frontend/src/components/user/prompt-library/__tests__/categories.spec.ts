import { describe, expect, it } from 'vitest'
import { getPromptLibraryCategory, matchesPromptLibraryCategory } from '../categories'

describe('prompt library categories', () => {
  it.each(['coding', 'writing', 'business', 'creative', 'education', 'workflow', 'productivity'] as const)(
    'keeps %s in the text catalog',
    (category) => {
      expect(getPromptLibraryCategory(category).type).toBe('TEXT')
    },
  )

  it('keeps image and video prompts in their own catalogs', () => {
    expect(getPromptLibraryCategory('image').type).toBe('IMAGE')
    expect(getPromptLibraryCategory('video').type).toBe('VIDEO')
  })

  it('uses the persisted AI category for textual catalogs', () => {
    expect(matchesPromptLibraryCategory({ ikikCategory: 'coding' }, 'coding')).toBe(true)
    expect(matchesPromptLibraryCategory({ ikikCategory: 'image' }, 'coding')).toBe(false)
    expect(matchesPromptLibraryCategory({ ikikCategory: 'video' }, 'all')).toBe(true)
  })
})
