import { apiClient } from '@/api/client'
import type {
  PromptAuditConfig,
  PromptAuditEvent,
  PromptAuditGroup,
  PromptAuditRuntime,
  PromptAuditUpdateRequest,
  PromptDeletePreview,
  PromptDeleteResult,
  PromptEventFilters,
  PromptEventPage,
  PromptProbeResult,
  PromptAuditTestResult,
  PromptAuditEndpointDraft,
	PromptAuditUserProfile,
	PromptAuditUserProfilePage,
	RiskKnowledgeEntryPage,
	RiskKnowledgeSummary,
	RiskKnowledgeWriteInput,
	RiskObservationPage,
	RiskObservationReviewInput,
	RiskObservation,
} from './types'
import { eventFilterPayload, eventQueryParams } from './viewModel'

const basePath = '/admin/prompt-audit'

export async function getConfig(): Promise<PromptAuditConfig> {
  const { data } = await apiClient.get<PromptAuditConfig>(`${basePath}/config`)
  return data
}

export async function updateConfig(payload: PromptAuditUpdateRequest): Promise<PromptAuditConfig> {
  const { data } = await apiClient.put<PromptAuditConfig>(`${basePath}/config`, payload)
  return data
}

export async function probeEndpoint(endpoint: PromptAuditEndpointDraft): Promise<PromptProbeResult> {
  const { data } = await apiClient.post<PromptProbeResult>(`${basePath}/endpoints/probe`, {
    endpoint: {
      id: endpoint.id,
      name: endpoint.name,
      protocol: 'openai_compatible',
      base_url: endpoint.base_url,
      model: endpoint.model,
      token: endpoint.token || undefined,
      timeout_ms: endpoint.timeout_ms,
      input_limit: endpoint.input_limit,
      enabled: endpoint.enabled,
    },
  })
  return data
}

export async function getRuntime(): Promise<PromptAuditRuntime> {
  const { data } = await apiClient.get<PromptAuditRuntime>(`${basePath}/runtime`)
  return data
}

export async function testPrompt(prompt: string): Promise<PromptAuditTestResult> {
  const { data } = await apiClient.post<PromptAuditTestResult>(`${basePath}/test`, { prompt })
  return data
}

export async function listEvents(
  filters: PromptEventFilters,
  page: number,
  pageSize: number,
): Promise<PromptEventPage> {
  const { data } = await apiClient.get<PromptEventPage>(`${basePath}/events`, {
    params: { page, page_size: pageSize, ...eventQueryParams(filters) },
  })
  return data
}

export async function getEvent(id: number): Promise<PromptAuditEvent> {
  const { data } = await apiClient.get<PromptAuditEvent>(`${basePath}/events/${id}`)
  return data
}

export async function deleteEvent(id: number): Promise<PromptDeleteResult> {
  const { data } = await apiClient.delete<PromptDeleteResult>(`${basePath}/events/${id}`)
  return data
}

export async function batchDeleteEvents(ids: number[]): Promise<PromptDeleteResult> {
  const { data } = await apiClient.post<PromptDeleteResult>(`${basePath}/events/batch-delete`, { ids })
  return data
}

export async function previewDelete(filters: PromptEventFilters): Promise<PromptDeletePreview> {
  const { data } = await apiClient.post<PromptDeletePreview>(
    `${basePath}/events/delete-preview`,
    eventFilterPayload(filters),
  )
  return data
}

export async function deleteEventsByFilter(
  filters: PromptEventFilters,
  preview: PromptDeletePreview,
): Promise<PromptDeleteResult> {
  const { data } = await apiClient.post<PromptDeleteResult>(`${basePath}/events/delete-by-filter`, {
    filter: eventFilterPayload(filters),
    snapshot_max_id: preview.snapshot_max_id,
    filter_hash: preview.filter_hash,
    confirmation_token: preview.confirmation_token,
    confirm: true,
  })
  return data
}

export async function listGroups(): Promise<PromptAuditGroup[]> {
  const { data } = await apiClient.get<PromptAuditGroup[]>('/admin/groups/all', {
    params: { include_inactive: true },
  })
  return data
}

export async function listProfiles(params: {
	page: number
	page_size: number
	blocked_only?: boolean
	keyword?: string
}): Promise<PromptAuditUserProfilePage> {
	const { data } = await apiClient.get<PromptAuditUserProfilePage>(`${basePath}/profiles`, { params })
	return data
}

export async function unblockProfile(userId: number): Promise<PromptAuditUserProfile> {
	const { data } = await apiClient.post<PromptAuditUserProfile>(`${basePath}/profiles/${userId}/unblock`)
	return data
}

export async function getKnowledgeSummary(): Promise<RiskKnowledgeSummary> {
  const { data } = await apiClient.get<RiskKnowledgeSummary>(`${basePath}/knowledge/summary`)
  return data
}

export async function listKnowledge(params: {
  page: number
  page_size: number
  topic?: string
  category?: string
  disposition?: string
  keyword?: string
  enabled?: boolean
}): Promise<RiskKnowledgeEntryPage> {
  const { data } = await apiClient.get<RiskKnowledgeEntryPage>(`${basePath}/knowledge`, { params })
  return data
}

export async function createKnowledge(input: RiskKnowledgeWriteInput): Promise<{ entry: RiskKnowledgeEntryPage['items'][number]; version: number }> {
  const { data } = await apiClient.post(`${basePath}/knowledge`, input)
  return data
}

export async function updateKnowledge(id: number, input: RiskKnowledgeWriteInput): Promise<{ entry: RiskKnowledgeEntryPage['items'][number]; version: number }> {
  const { data } = await apiClient.put(`${basePath}/knowledge/${id}`, input)
  return data
}

export async function listKnowledgeObservations(params: {
  page: number
  page_size: number
  review_status?: string
  category?: string
  user_id?: number
  group_id?: number
  keyword?: string
}): Promise<RiskObservationPage> {
  const { data } = await apiClient.get<RiskObservationPage>(`${basePath}/knowledge/observations`, { params })
  return data
}

export async function reviewKnowledgeObservation(id: number, input: RiskObservationReviewInput): Promise<RiskObservation> {
  const { data } = await apiClient.put<RiskObservation>(`${basePath}/knowledge/observations/${id}/review`, input)
  return data
}

export const promptAuditAPI = {
  getConfig,
  updateConfig,
  probeEndpoint,
  getRuntime,
  testPrompt,
  listEvents,
  getEvent,
  deleteEvent,
  batchDeleteEvents,
  previewDelete,
  deleteEventsByFilter,
  listGroups,
	listProfiles,
	unblockProfile,
	getKnowledgeSummary,
	listKnowledge,
	createKnowledge,
	updateKnowledge,
	listKnowledgeObservations,
	reviewKnowledgeObservation,
}

export default promptAuditAPI
