import type {
  UserAvailableChannel,
  UserAvailableGroup,
  UserSupportedModel
} from '@/api/channels'

export type ModelMarketPricingFilter = 'all' | 'with' | 'without'
export type ModelMarketCategoryKey =
  | 'all'
  | 'openai'
  | 'anthropic'
  | 'gemini'
  | 'qwen'
  | 'deepseek'
  | 'zhipu'
  | 'image'
  | 'embedding'
  | 'other'

export interface ModelMarketChannelRef {
  name: string
  description: string
}

export interface ModelMarketItem {
  group: UserAvailableGroup
  platform: string
  channels: ModelMarketChannelRef[]
  models: ModelMarketModel[]
  channel_count: number
  model_count: number
  has_pricing: boolean
}

export interface ModelMarketModel extends UserSupportedModel {
  channels: ModelMarketChannelRef[]
  has_pricing: boolean
  pricing_conflict: boolean
}

export interface ModelMarketFilters {
  search: string
  category?: ModelMarketCategoryKey
  platform: string
  channel: string
  pricing: ModelMarketPricingFilter
}

export interface ModelMarketFilterOptions {
  platforms: string[]
  channels: string[]
}

export interface ModelMarketCatalogItem {
  key: string
  name: string
  platform: string
  category: Exclude<ModelMarketCategoryKey, 'all'>
  model: ModelMarketModel
  groups: UserAvailableGroup[]
  channels: ModelMarketChannelRef[]
  group_count: number
  channel_count: number
  has_pricing: boolean
  pricing_conflict: boolean
}

interface ModelAccumulator extends ModelMarketItem {
  modelPricingKeys: Map<string, string | null>
}

interface CatalogAccumulator extends ModelMarketCatalogItem {
  groupIds: Set<number>
  channelNames: Set<string>
  pricingKeys: Set<string>
}

export function buildModelMarketItems(channels: UserAvailableChannel[]): ModelMarketItem[] {
  const byGroup = new Map<number, ModelAccumulator>()

  for (const channel of channels) {
    for (const section of channel.platforms) {
      for (const group of section.groups) {
        const item = getOrCreateGroupItem(byGroup, group, section.platform)
        addChannel(item, channel)
        addModels(item, section.supported_models, section.platform, channel)
      }
    }
  }

  return Array.from(byGroup.values())
    .map(finalizeGroupItem)
    .filter((item) => item.model_count > 0)
    .sort(compareModelMarketItems)
}

export function buildModelCatalogItems(channels: UserAvailableChannel[]): ModelMarketCatalogItem[] {
  const byModel = new Map<string, CatalogAccumulator>()

  for (const channel of channels) {
    for (const section of channel.platforms) {
      for (const model of section.supported_models) {
        const platform = model.platform || section.platform || ''
        const key = modelKey(platform, model.name)
        const item = getOrCreateCatalogItem(byModel, key, model, platform)
        addCatalogChannel(item, channel)
        addCatalogGroups(item, section.groups)
        mergeCatalogPricing(item, model, platform, channel)
      }
    }
  }

  return Array.from(byModel.values())
    .map(finalizeCatalogItem)
    .sort(compareModelCatalogItems)
}

export function filterModelMarketItems(items: ModelMarketItem[], filters: ModelMarketFilters): ModelMarketItem[] {
  const search = filters.search.trim().toLowerCase()
  return items
    .map((item) => filterGroupItem(item, filters, search))
    .filter((item): item is ModelMarketItem => item !== null)
}

export function filterModelCatalogItems(
  items: ModelMarketCatalogItem[],
  filters: ModelMarketFilters
): ModelMarketCatalogItem[] {
  const search = filters.search.trim().toLowerCase()
  return items
    .map((item) => filterCatalogItem(item, filters, search))
    .filter((item): item is ModelMarketCatalogItem => item !== null)
}

export function getModelMarketFilterOptions(items: ModelMarketItem[]): ModelMarketFilterOptions {
  return {
    platforms: sortStrings(unique(items.map((item) => item.platform).filter(Boolean))),
    channels: sortStrings(unique(items.flatMap((item) => item.channels.map((channel) => channel.name))))
  }
}

export function getModelCatalogFilterOptions(items: ModelMarketCatalogItem[]): ModelMarketFilterOptions {
  return {
    platforms: sortStrings(unique(items.map((item) => item.platform).filter(Boolean))),
    channels: sortStrings(unique(items.flatMap((item) => item.channels.map((channel) => channel.name))))
  }
}

export function countModelMarketModels(items: ModelMarketItem[]): number {
  return items.reduce((count, item) => count + item.models.length, 0)
}

export function countModelCatalogGroups(items: ModelMarketCatalogItem[]): number {
  const groupIds = new Set<number>()
  for (const item of items) {
    for (const group of item.groups) groupIds.add(group.id)
  }
  return groupIds.size
}

export function countModelCatalogChannels(items: ModelMarketCatalogItem[]): number {
  const channelNames = new Set<string>()
  for (const item of items) {
    for (const channel of item.channels) channelNames.add(channel.name)
  }
  return channelNames.size
}

function getOrCreateGroupItem(
  byGroup: Map<number, ModelAccumulator>,
  group: UserAvailableGroup,
  platform: string
): ModelAccumulator {
  let item = byGroup.get(group.id)
  if (!item) {
    item = {
      group,
      platform: group.platform || platform,
      channels: [],
      models: [],
      channel_count: 0,
      model_count: 0,
      has_pricing: false,
      modelPricingKeys: new Map()
    }
    byGroup.set(group.id, item)
  }
  return item
}

function addChannel(item: ModelMarketItem, channel: UserAvailableChannel) {
  if (item.channels.some((existing) => existing.name === channel.name)) return
  item.channels.push({
    name: channel.name,
    description: channel.description || ''
  })
}

function addModels(
  item: ModelAccumulator,
  models: UserSupportedModel[],
  platform: string,
  channel: UserAvailableChannel
) {
  for (const model of models) {
    const normalizedModel = {
      ...model,
      platform: model.platform || platform,
      channels: [createChannelRef(channel)],
      pricing_conflict: false,
      has_pricing: model.pricing != null
    }
    const key = modelKey(normalizedModel.platform, normalizedModel.name)
    const existingIndex = item.models.findIndex((existing) => modelKey(existing.platform, existing.name) === key)
    if (existingIndex === -1) {
      item.models.push(normalizedModel)
      item.modelPricingKeys.set(key, pricingKey(normalizedModel))
    } else {
      const existing = item.models[existingIndex]
      addModelChannel(existing, channel)
      const previousPricingKey = item.modelPricingKeys.get(key) ?? null
      const nextPricingKey = pricingKey(normalizedModel)
      if (previousPricingKey !== nextPricingKey) {
        item.models[existingIndex] = {
          ...existing,
          pricing: null,
          has_pricing: existing.has_pricing || normalizedModel.has_pricing,
          pricing_conflict: true
        }
        item.modelPricingKeys.set(key, null)
      } else if (existing.pricing == null && normalizedModel.pricing != null) {
        item.models[existingIndex] = {
          ...existing,
          ...normalizedModel,
          channels: existing.channels,
          has_pricing: true
        }
      }
    }
    if (normalizedModel.has_pricing) {
      item.has_pricing = true
    }
  }
}

function getOrCreateCatalogItem(
  byModel: Map<string, CatalogAccumulator>,
  key: string,
  model: UserSupportedModel,
  platform: string
): CatalogAccumulator {
  let item = byModel.get(key)
  if (!item) {
    item = {
      key,
      name: model.name,
      platform,
      category: inferModelCategory(model.name, platform),
      model: {
        ...model,
        platform,
        channels: [],
        has_pricing: model.pricing != null,
        pricing_conflict: false
      },
      groups: [],
      channels: [],
      group_count: 0,
      channel_count: 0,
      has_pricing: model.pricing != null,
      pricing_conflict: false,
      groupIds: new Set<number>(),
      channelNames: new Set<string>(),
      pricingKeys: new Set<string>()
    }
    byModel.set(key, item)
  }
  return item
}

function addCatalogChannel(item: CatalogAccumulator, channel: UserAvailableChannel) {
  if (item.channelNames.has(channel.name)) return
  item.channelNames.add(channel.name)
  item.channels.push(createChannelRef(channel))
}

function addCatalogGroups(item: CatalogAccumulator, groups: UserAvailableGroup[]) {
  for (const group of groups) {
    if (item.groupIds.has(group.id)) continue
    item.groupIds.add(group.id)
    item.groups.push(group)
  }
}

function mergeCatalogPricing(
  item: CatalogAccumulator,
  model: UserSupportedModel,
  platform: string,
  channel: UserAvailableChannel
) {
  addModelChannel(item.model, channel)
  const key = pricingKey({ ...model, platform })
  if (key != null) {
    item.pricingKeys.add(key)
    item.has_pricing = true
    item.model.has_pricing = true
  }
  if (item.pricingKeys.size > 1) {
    item.pricing_conflict = true
    item.model.pricing_conflict = true
    item.model.pricing = null
  } else if (item.model.pricing == null && model.pricing != null) {
    item.model.pricing = model.pricing
  }
}

function finalizeGroupItem(item: ModelAccumulator): ModelMarketItem {
  return {
    group: item.group,
    platform: item.platform,
    channels: sortChannels(item.channels),
    models: sortModels(item.models.map((model) => ({ ...model, channels: sortChannels(model.channels) }))),
    channel_count: item.channels.length,
    model_count: item.models.length,
    has_pricing: item.has_pricing
  }
}

function finalizeCatalogItem(item: CatalogAccumulator): ModelMarketCatalogItem {
  const groups = sortGroups(item.groups)
  const channels = sortChannels(item.channels)
  return {
    key: item.key,
    name: item.name,
    platform: item.platform,
    category: item.category,
    model: {
      ...item.model,
      channels,
      has_pricing: item.has_pricing,
      pricing_conflict: item.pricing_conflict
    },
    groups,
    channels,
    group_count: groups.length,
    channel_count: channels.length,
    has_pricing: item.has_pricing,
    pricing_conflict: item.pricing_conflict
  }
}

function filterGroupItem(
  item: ModelMarketItem,
  filters: ModelMarketFilters,
  search: string
): ModelMarketItem | null {
  if (filters.platform && item.platform !== filters.platform) return null

  const models = item.models.filter((model) => modelMatches(model, item, filters, search))
  if (models.length === 0) return null
  const channels = channelsForModels(item.channels, models, filters.channel)

  return {
    ...item,
    channels,
    models,
    channel_count: channels.length,
    model_count: models.length,
    has_pricing: models.some((model) => model.has_pricing)
  }
}

function filterCatalogItem(
  item: ModelMarketCatalogItem,
  filters: ModelMarketFilters,
  search: string
): ModelMarketCatalogItem | null {
  if (filters.category && filters.category !== 'all' && item.category !== filters.category) return null
  if (filters.platform && item.platform !== filters.platform) return null
  if (filters.channel && !item.channels.some((channel) => channel.name === filters.channel)) return null
  if (filters.pricing === 'with' && !item.has_pricing) return null
  if (filters.pricing === 'without' && item.has_pricing) return null
  if (!search) return item

  const matches =
    item.name.toLowerCase().includes(search) ||
    item.platform.toLowerCase().includes(search) ||
    item.channels.some((channel) =>
      channel.name.toLowerCase().includes(search) ||
      channel.description.toLowerCase().includes(search)
    ) ||
    item.groups.some((group) =>
      group.name.toLowerCase().includes(search) ||
      group.platform.toLowerCase().includes(search)
    )

  return matches ? item : null
}

function modelMatches(
  model: ModelMarketModel,
  item: ModelMarketItem,
  filters: ModelMarketFilters,
  search: string
): boolean {
  if (filters.channel && !model.channels.some((channel) => channel.name === filters.channel)) return false
  if (filters.pricing === 'with' && !model.has_pricing) return false
  if (filters.pricing === 'without' && model.has_pricing) return false
  if (!search) return true

  return (
    item.group.name.toLowerCase().includes(search) ||
    item.platform.toLowerCase().includes(search) ||
    model.name.toLowerCase().includes(search) ||
    model.platform.toLowerCase().includes(search) ||
    model.channels.some((channel) =>
      channel.name.toLowerCase().includes(search) ||
      channel.description.toLowerCase().includes(search)
    )
  )
}

function channelsForModels(
  channels: ModelMarketChannelRef[],
  models: ModelMarketModel[],
  channelFilter: string
): ModelMarketChannelRef[] {
  const modelChannelNames = new Set(models.flatMap((model) => model.channels.map((channel) => channel.name)))
  return channels.filter((channel) =>
    (!channelFilter || channel.name === channelFilter) &&
    modelChannelNames.has(channel.name)
  )
}

function sortChannels(channels: ModelMarketChannelRef[]): ModelMarketChannelRef[] {
  return [...channels].sort((a, b) => compareStrings(a.name, b.name))
}

function createChannelRef(channel: UserAvailableChannel): ModelMarketChannelRef {
  return {
    name: channel.name,
    description: channel.description || ''
  }
}

function addModelChannel(model: ModelMarketModel, channel: UserAvailableChannel) {
  if (model.channels.some((existing) => existing.name === channel.name)) return
  model.channels.push(createChannelRef(channel))
}

function sortModels(models: ModelMarketModel[]): ModelMarketModel[] {
  return [...models].sort((a, b) => {
    const platform = compareStrings(a.platform, b.platform)
    if (platform !== 0) return platform
    return compareStrings(a.name, b.name)
  })
}

function sortGroups(groups: UserAvailableGroup[]): UserAvailableGroup[] {
  return [...groups].sort((a, b) => {
    if (a.is_exclusive !== b.is_exclusive) return a.is_exclusive ? -1 : 1
    return compareStrings(a.name, b.name)
  })
}

function compareModelMarketItems(a: ModelMarketItem, b: ModelMarketItem): number {
  const platform = compareStrings(a.platform, b.platform)
  if (platform !== 0) return platform
  if (a.group.is_exclusive !== b.group.is_exclusive) return a.group.is_exclusive ? -1 : 1
  return compareStrings(a.group.name, b.group.name)
}

function compareModelCatalogItems(a: ModelMarketCatalogItem, b: ModelMarketCatalogItem): number {
  const category = categoryRank(a.category) - categoryRank(b.category)
  if (category !== 0) return category
  const platform = compareStrings(a.platform, b.platform)
  if (platform !== 0) return platform
  return compareStrings(a.name, b.name)
}

function categoryRank(category: ModelMarketCategoryKey): number {
  const order: ModelMarketCategoryKey[] = [
    'openai',
    'anthropic',
    'gemini',
    'qwen',
    'deepseek',
    'zhipu',
    'image',
    'embedding',
    'other'
  ]
  const index = order.indexOf(category)
  return index === -1 ? order.length : index
}

function modelKey(platform: string, model: string): string {
  return `${platform.trim().toLowerCase()}::${model.trim().toLowerCase()}`
}

function inferModelCategory(model: string, platform: string): Exclude<ModelMarketCategoryKey, 'all'> {
  const text = `${platform} ${model}`.toLowerCase()
  if (
    text.includes('embedding') ||
    text.includes('embed') ||
    text.includes('text-embedding') ||
    text.includes('bge-') ||
    text.includes('jina')
  ) {
    return 'embedding'
  }
  if (
    text.includes('image') ||
    text.includes('dall-e') ||
    text.includes('gpt-image') ||
    text.includes('imagen') ||
    text.includes('cogview') ||
    text.includes('flux') ||
    text.includes('midjourney') ||
    text.includes('mj_')
  ) {
    return 'image'
  }
  if (
    text.includes('openai') ||
    text.includes('gpt') ||
    text.includes('o1') ||
    text.includes('o3') ||
    text.includes('o4') ||
    text.includes('codex')
  ) {
    return 'openai'
  }
  if (text.includes('anthropic') || text.includes('claude') || text.includes('opus') || text.includes('sonnet')) {
    return 'anthropic'
  }
  if (text.includes('gemini') || text.includes('gemma') || text.includes('google')) {
    return 'gemini'
  }
  if (text.includes('qwen') || text.includes('通义') || text.includes('dashscope')) {
    return 'qwen'
  }
  if (text.includes('deepseek')) {
    return 'deepseek'
  }
  if (text.includes('zhipu') || text.includes('glm') || text.includes('chatglm')) {
    return 'zhipu'
  }
  return 'other'
}

function pricingKey(model: UserSupportedModel): string | null {
  return model.pricing == null ? null : JSON.stringify(model.pricing)
}

function unique(values: string[]): string[] {
  return Array.from(new Set(values))
}

function sortStrings(values: string[]): string[] {
  return [...values].sort(compareStrings)
}

function compareStrings(a: string, b: string): number {
  return a.localeCompare(b, 'zh-Hans-CN')
}
