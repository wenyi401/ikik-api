export type PromptLibraryCategoryKey =
  | 'featured'
  | 'community'
  | 'coding'
  | 'writing'
  | 'business'
  | 'creative'
  | 'image'
  | 'video'
  | 'education'
  | 'workflow'
  | 'productivity'
  | 'all'

export interface PromptLibraryCategory {
  key: PromptLibraryCategoryKey
  query?: string
  type?: 'TEXT' | 'IMAGE' | 'VIDEO'
  sort?: 'newest' | 'upvotes'
}

export const promptLibraryCategories: PromptLibraryCategory[] = [
  { key: 'featured', sort: 'upvotes' },
  { key: 'community', sort: 'newest' },
  { key: 'coding', query: 'coding', type: 'TEXT', sort: 'upvotes' },
  { key: 'writing', query: 'writing', type: 'TEXT', sort: 'upvotes' },
  { key: 'business', query: 'business', type: 'TEXT', sort: 'upvotes' },
  { key: 'creative', query: 'creative', type: 'TEXT', sort: 'upvotes' },
  { key: 'image', type: 'IMAGE', sort: 'upvotes' },
  { key: 'video', type: 'VIDEO', sort: 'upvotes' },
  { key: 'education', query: 'education', type: 'TEXT', sort: 'upvotes' },
  { key: 'workflow', query: 'workflow', type: 'TEXT', sort: 'upvotes' },
  { key: 'productivity', query: 'productivity', type: 'TEXT', sort: 'upvotes' },
  { key: 'all', sort: 'newest' },
]

export function getPromptLibraryCategory(key: PromptLibraryCategoryKey): PromptLibraryCategory {
  return promptLibraryCategories.find(category => category.key === key) || promptLibraryCategories[0]
}

export function matchesPromptLibraryCategory(
  item: { ikikCategory?: string },
  category: PromptLibraryCategoryKey,
): boolean {
  if (category === 'featured' || category === 'community' || category === 'all') return true
  return item.ikikCategory === category
}
