<template>
  <AppLayout>
    <UiPage width="wide" density="compact">
      <UiPageHeader :title="t('pet.hallTitle')" :description="t('pet.hallDescription')">
        <template #actions>
          <a
            class="pet-source-link"
            href="https://github.com/legeling/awesome-codex-pet"
            target="_blank"
            rel="noreferrer"
          >
            <span>awesome-codex-pet</span>
            <Icon name="externalLink" size="xs" />
          </a>
          <label class="btn btn-secondary btn-sm cursor-pointer" :class="{ 'pointer-events-none opacity-50': busy }">
            <Icon name="upload" size="sm" />
            {{ busy ? t('common.processing') : t('pet.importZip') }}
            <input class="sr-only" type="file" accept=".zip,application/zip" :disabled="busy" @change="importZip" />
          </label>
        </template>
      </UiPageHeader>

      <section v-if="pet.initialized" class="pet-control-band" :aria-label="t('pet.controlsTitle')">
        <div class="pet-control-band__identity">
          <div class="pet-control-band__preview">
            <PetHallPreview v-if="pet.selectedAsset" :key="pet.selectedAsset.id" :asset="pet.selectedAsset" />
            <Icon v-else name="chat" size="xl" />
          </div>
          <div class="min-w-0">
            <p class="pet-control-band__eyebrow">{{ t('pet.currentPet') }}</p>
            <h2>{{ pet.selectedAsset?.display_name || t('pet.noPetSelected') }}</h2>
          </div>
        </div>

        <div class="pet-control-band__settings">
          <div class="pet-setting pet-setting--toggle">
            <div>
              <span>{{ t('pet.enabled') }}</span>
              <small>{{ pet.preferences.enabled ? t('pet.enabledState') : t('pet.disabledState') }}</small>
            </div>
            <Toggle :model-value="pet.preferences.enabled" @update:model-value="setEnabled" />
          </div>

          <label class="pet-setting">
            <span>{{ t('pet.size') }}</span>
            <select :value="pet.preferences.size" :disabled="busy" @change="selectSize">
              <option value="small">{{ t('pet.sizes.small') }}</option>
              <option value="medium">{{ t('pet.sizes.medium') }}</option>
              <option value="large">{{ t('pet.sizes.large') }}</option>
            </select>
          </label>

          <label class="pet-setting">
            <span>{{ t('pet.position') }}</span>
            <select :value="pet.preferences.anchor" :disabled="busy" @change="selectAnchor">
              <option value="bottom-left">{{ t('pet.positions.left') }}</option>
              <option value="bottom-right">{{ t('pet.positions.right') }}</option>
            </select>
          </label>

          <label class="pet-setting pet-setting--wide">
            <span>
              {{ t('pet.assistantGroup') }}
              <small>{{ t('pet.assistantGroupHint') }}</small>
            </span>
            <select data-testid="assistant-group-select" :value="pet.preferences.assistant_group_id || ''" :disabled="busy || groupsLoading" @change="selectAssistantGroup">
              <option value="">{{ t('pet.selectAssistantGroup') }}</option>
              <option v-for="group in groups" :key="group.id" :value="group.id">
                {{ group.name }}
              </option>
            </select>
          </label>

          <div class="pet-setting pet-setting--toggle">
            <span>{{ t('pet.activityReactions') }}</span>
            <Toggle :model-value="pet.preferences.activity_reactions" @update:model-value="setActivity" />
          </div>

          <div class="pet-setting pet-setting--toggle">
            <span>{{ t('pet.reducedMotion') }}</span>
            <Toggle :model-value="pet.preferences.reduced_motion" @update:model-value="setReducedMotion" />
          </div>
        </div>
      </section>

      <section class="pet-library" :aria-label="t('pet.libraryTitle')">
        <header class="pet-library__toolbar">
          <div>
            <h2>{{ t('pet.libraryTitle') }}</h2>
            <p>{{ t('pet.assetCount', { count: pet.assets.length }) }}</p>
          </div>
          <label class="pet-search">
            <Icon name="search" size="sm" />
            <input v-model.trim="query" type="search" :placeholder="t('pet.searchPlaceholder')" />
            <button v-if="query" type="button" :aria-label="t('common.clear')" @click="query = ''">
              <Icon name="x" size="xs" />
            </button>
          </label>
        </header>

        <div v-if="pet.loading && !pet.initialized" class="pet-grid" aria-busy="true">
          <div v-for="index in 8" :key="index" class="pet-card-skeleton">
            <span /><span /><span />
          </div>
        </div>

        <div v-else-if="loadFailed" class="pet-empty-state">
          <Icon name="exclamationCircle" size="lg" />
          <p>{{ t('pet.loadFailed') }}</p>
          <button type="button" class="btn btn-secondary btn-sm" @click="initialize">{{ t('common.tryAgain') }}</button>
        </div>

        <div v-else-if="filteredAssets.length === 0" class="pet-empty-state">
          <Icon name="search" size="lg" />
          <p>{{ query ? t('pet.searchEmpty') : t('pet.libraryEmpty') }}</p>
        </div>

        <div v-else class="pet-grid">
          <article
            v-for="asset in filteredAssets"
            :key="asset.id"
            data-testid="pet-card"
            class="pet-card"
            :class="{
              'pet-card--selected': asset.id === pet.preferences.selected_asset_id,
              'pet-card--saving': selectingID === asset.id,
            }"
          >
            <button
              type="button"
              class="pet-card__select"
              :aria-pressed="asset.id === pet.preferences.selected_asset_id"
              :aria-label="t('pet.selectPet', { name: asset.display_name })"
              :disabled="busy"
              @click="selectAsset(asset)"
            >
              <PetHallPreview :asset="asset" />
              <span class="pet-card__copy">
                <strong :title="asset.display_name">{{ asset.display_name }}</strong>
              </span>
              <span v-if="asset.id === pet.preferences.selected_asset_id" class="pet-card__selected-mark">
                <Icon name="check" size="xs" />
                {{ t('pet.selected') }}
              </span>
            </button>
            <button
              v-if="!asset.is_builtin"
              type="button"
              class="pet-card__delete"
              :title="t('pet.deletePet')"
              :aria-label="t('pet.deletePet')"
              :disabled="busy"
              @click="removeAsset(asset)"
            >
              <Icon name="trash" size="sm" />
            </button>
          </article>
        </div>
      </section>
    </UiPage>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { PetAsset, PetPreferences } from '@/api/pet'
import userGroupsAPI from '@/api/groups'
import Icon from '@/components/icons/Icon.vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import Toggle from '@/components/common/Toggle.vue'
import PetHallPreview from '@/features/pet/PetHallPreview.vue'
import { useAppStore } from '@/stores/app'
import { usePetStore } from '@/stores/pet'
import { UiPage, UiPageHeader } from '@/ui'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { Group } from '@/types'

const { t } = useI18n()
const pet = usePetStore()
const app = useAppStore()
const query = ref('')
const busy = ref(false)
const selectingID = ref('')
const loadFailed = ref(false)
const groups = ref<Group[]>([])
const groupsLoading = ref(false)

const filteredAssets = computed(() => {
  const needle = query.value.toLocaleLowerCase()
  if (!needle) return pet.assets
  return pet.assets.filter((asset) => (
    `${asset.display_name} ${asset.pet_key} ${asset.description}`.toLocaleLowerCase().includes(needle)
  ))
})

async function initialize() {
  loadFailed.value = false
  try {
    await pet.initialize()
  } catch (error) {
    loadFailed.value = true
    app.showError(extractApiErrorMessage(error, t('pet.loadFailed')))
  }
}

async function persist(patch: Partial<PetPreferences>) {
  if (busy.value) return
  busy.value = true
  try {
    await pet.savePreferences({ ...pet.preferences, ...patch })
  } catch (error) {
    app.showError(extractApiErrorMessage(error, t('pet.saveFailed')))
  } finally {
    busy.value = false
  }
}

function setEnabled(value: boolean) { void persist({ enabled: value }) }
function setActivity(value: boolean) { void persist({ activity_reactions: value }) }
function setReducedMotion(value: boolean) { void persist({ reduced_motion: value }) }
function selectSize(event: Event) {
  void persist({ size: (event.target as HTMLSelectElement).value as PetPreferences['size'] })
}
function selectAnchor(event: Event) {
  void persist({
    anchor: (event.target as HTMLSelectElement).value as PetPreferences['anchor'],
    position_x: null,
    position_y: null,
  })
}
function selectAssistantGroup(event: Event) {
  const value = Number((event.target as HTMLSelectElement).value)
  void persist({ assistant_group_id: value > 0 ? value : null, assistant_model: '' })
}

async function loadGroups() {
  groupsLoading.value = true
  try {
    groups.value = (await userGroupsAPI.getAvailable()).filter((group) => !group.claude_code_only)
  } catch (error) {
    groups.value = []
    app.showError(extractApiErrorMessage(error, t('pet.groupsLoadFailed')))
  } finally {
    groupsLoading.value = false
  }
}

async function selectAsset(asset: PetAsset) {
  if (asset.id === pet.preferences.selected_asset_id || busy.value) return
  selectingID.value = asset.id
  await persist({ selected_asset_id: asset.id })
  selectingID.value = ''
}

async function importZip(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file || busy.value) return
  busy.value = true
  try {
    await pet.importAsset(file)
    app.showSuccess(t('pet.imported'))
  } catch (error) {
    app.showError(extractApiErrorMessage(error, t('pet.importFailed')))
  } finally {
    busy.value = false
  }
}

async function removeAsset(asset: PetAsset) {
  if (asset.is_builtin || busy.value) return
  busy.value = true
  try {
    await pet.removeAsset(asset.id)
    app.showSuccess(t('pet.deleted'))
  } catch (error) {
    app.showError(extractApiErrorMessage(error, t('pet.deleteFailed')))
  } finally {
    busy.value = false
  }
}

onMounted(() => {
  if (!pet.initialized) void initialize()
  void loadGroups()
})
</script>

<style scoped>
.pet-source-link {
  display: inline-flex;
  min-height: 2rem;
  align-items: center;
  gap: 0.35rem;
  color: var(--ui-text-secondary);
  font-size: 0.75rem;
  transition: color 180ms ease;
}

.pet-source-link:hover { color: var(--ui-text); }

.pet-control-band {
  display: grid;
  grid-template-columns: minmax(15rem, 0.9fr) minmax(26rem, 1.6fr);
  overflow: hidden;
  border: 1px solid var(--ui-border);
  border-radius: 0.5rem;
  background: var(--ui-surface);
}

.pet-control-band__identity {
  display: grid;
  grid-template-columns: 7rem minmax(0, 1fr);
  align-items: center;
  gap: 1rem;
  min-width: 0;
  padding: 1rem;
  border-right: 1px solid var(--ui-border);
}

.pet-control-band__preview {
  display: grid;
  height: 7rem;
  overflow: hidden;
  place-items: center;
  border-radius: 0.375rem;
  color: var(--ui-text-tertiary);
}

.pet-control-band__preview :deep(.pet-hall-preview) { min-height: 7rem; }

.pet-control-band__eyebrow {
  margin-bottom: 0.25rem;
  color: var(--ui-text-tertiary);
  font-size: 0.6875rem;
  font-weight: 600;
}

.pet-control-band__identity h2 {
  overflow: hidden;
  color: var(--ui-text);
  font-size: 1rem;
  font-weight: 600;
  line-height: 1.35;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pet-control-band__settings {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  align-content: center;
  gap: 0.75rem 1rem;
  padding: 1rem;
}

.pet-setting {
  display: flex;
  min-width: 0;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  color: var(--ui-text-secondary);
  font-size: 0.8125rem;
}

.pet-setting--toggle { min-height: 2rem; }
.pet-setting--wide { grid-column: 1 / -1; }
.pet-setting--wide select { width: min(100%, 18rem); }

.pet-setting small {
  display: block;
  margin-top: 0.1rem;
  color: var(--ui-text-tertiary);
  font-size: 0.6875rem;
}

.pet-setting select {
  width: 8.25rem;
  height: 2rem;
  border: 1px solid var(--ui-border);
  border-radius: 0.375rem;
  background: var(--ui-surface-subtle);
  padding: 0 1.75rem 0 0.625rem;
  color: var(--ui-text);
  font-size: 0.75rem;
}

.pet-setting select:focus-visible,
.pet-search:focus-within,
.pet-card__select:focus-visible,
.pet-card__delete:focus-visible,
.pet-source-link:focus-visible {
  outline: 2px solid var(--app-primary);
  outline-offset: 2px;
}

.pet-library {
  min-width: 0;
}

.pet-library__toolbar {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 1rem;
  margin-bottom: 0.75rem;
}

.pet-library__toolbar h2 {
  color: var(--ui-text);
  font-size: 1rem;
  font-weight: 600;
}

.pet-library__toolbar p {
  margin-top: 0.15rem;
  color: var(--ui-text-tertiary);
  font-size: 0.75rem;
}

.pet-search {
  display: flex;
  width: min(100%, 17rem);
  height: 2.25rem;
  align-items: center;
  gap: 0.5rem;
  border: 1px solid var(--ui-border);
  border-radius: 0.375rem;
  background: var(--ui-surface);
  padding: 0 0.625rem;
  color: var(--ui-text-tertiary);
}

.pet-search input {
  min-width: 0;
  flex: 1;
  border: 0;
  background: transparent;
  color: var(--ui-text);
  font-size: 0.8125rem;
  outline: 0;
}

.pet-search button {
  display: grid;
  width: 1.5rem;
  height: 1.5rem;
  place-items: center;
  border-radius: 0.25rem;
}

.pet-search button:hover { background: var(--ui-surface-hover); }

.pet-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(10rem, 1fr));
  gap: 0.75rem;
}

.pet-card {
  position: relative;
  min-width: 0;
  overflow: hidden;
  border: 1px solid var(--ui-border);
  border-radius: 0.5rem;
  background: var(--ui-surface);
  transition: border-color 180ms ease, transform 180ms ease, background-color 180ms ease;
}

.pet-card:hover {
  transform: translateY(-1px);
  border-color: var(--ui-border-strong);
}

.pet-card:active { transform: translateY(0); }

.pet-card--selected {
  border-color: var(--app-primary);
  box-shadow: inset 0 0 0 1px var(--app-primary);
}

.pet-card--saving { opacity: 0.68; }

.pet-card__select {
  display: block;
  width: 100%;
  min-width: 0;
  text-align: left;
}

.pet-card__select :deep(.pet-hall-preview) { min-height: 7.5rem; }

.pet-card__copy {
  display: block;
  min-width: 0;
  padding: 0.7rem 0.75rem 0.75rem;
}

.pet-card__copy strong {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pet-card__copy strong {
  padding-right: 0.25rem;
  color: var(--ui-text);
  font-size: 0.8125rem;
  font-weight: 600;
  line-height: 1.4;
}

.pet-card__selected-mark {
  position: absolute;
  top: 0.5rem;
  left: 0.5rem;
  display: inline-flex;
  align-items: center;
  gap: 0.2rem;
  border-radius: 0.25rem;
  background: var(--app-primary);
  padding: 0.2rem 0.35rem;
  color: white;
  font-size: 0.625rem;
  font-weight: 600;
}

.pet-card__delete {
  position: absolute;
  right: 0.45rem;
  bottom: 0.5rem;
  display: grid;
  width: 1.75rem;
  height: 1.75rem;
  place-items: center;
  border-radius: 0.25rem;
  color: #dc2626;
}

.pet-card__delete:hover { background: color-mix(in srgb, #dc2626 10%, transparent); }

.pet-card-skeleton {
  height: 11.25rem;
  overflow: hidden;
  border: 1px solid var(--ui-border);
  border-radius: 0.5rem;
  background: var(--ui-surface);
  padding: 0.75rem;
}

.pet-card-skeleton span {
  display: block;
  border-radius: 0.25rem;
  background: var(--ui-surface-hover);
}

.pet-card-skeleton span:first-child { height: 7.5rem; }
.pet-card-skeleton span:nth-child(2) { width: 72%; height: 0.75rem; margin-top: 0.65rem; }
.pet-card-skeleton span:last-child { width: 45%; height: 0.55rem; margin-top: 0.4rem; }

.pet-empty-state {
  display: grid;
  min-height: 14rem;
  place-items: center;
  align-content: center;
  gap: 0.75rem;
  border: 1px dashed var(--ui-border-strong);
  border-radius: 0.5rem;
  color: var(--ui-text-tertiary);
  text-align: center;
}

@media (max-width: 900px) {
  .pet-control-band { grid-template-columns: 1fr; }
  .pet-control-band__identity { border-right: 0; border-bottom: 1px solid var(--ui-border); }
}

@media (max-width: 640px) {
  .pet-source-link { display: none; }
  .pet-control-band__identity { grid-template-columns: 5.5rem minmax(0, 1fr); }
  .pet-control-band__preview { height: 5.5rem; }
  .pet-control-band__preview :deep(.pet-hall-preview) { min-height: 5.5rem; }
  .pet-control-band__settings { grid-template-columns: 1fr; }
  .pet-library__toolbar { align-items: stretch; flex-direction: column; }
  .pet-search { width: 100%; }
  .pet-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 0.5rem; }
  .pet-card__copy { padding-inline: 0.6rem; }
  .pet-card__selected-mark { max-width: calc(100% - 1rem); }
}
</style>
