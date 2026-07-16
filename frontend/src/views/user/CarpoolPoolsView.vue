<template>
  <AppLayout>
    <UiPage width="wide">
      <section class="carpool-command-bar">
        <div class="carpool-tabs" role="tablist" :aria-label="t('carpool.title')">
          <button
            type="button"
            role="tab"
            :aria-selected="activeTab === 'mine'"
            :class="['carpool-tab', { 'carpool-tab--active': activeTab === 'mine' }]"
            @click="activeTab = 'mine'"
          >
            {{ t('carpool.myPools') }}
          </button>
          <button
            type="button"
            role="tab"
            :aria-selected="activeTab === 'invite'"
            :class="['carpool-tab', { 'carpool-tab--active': activeTab === 'invite' }]"
            @click="activeTab = 'invite'"
          >
            {{ t('carpool.joinByInvite') }}
          </button>
        </div>
        <div class="carpool-actions">
          <UiIconButton :label="t('common.refresh')" :disabled="loading" @click="loadOverview">
            <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
          </UiIconButton>
          <button
            type="button"
            class="btn btn-primary"
            @click="openCreateDialog"
          >
            <Icon name="plus" size="md" class="mr-2" />
            {{ t('carpool.createPool') }}
          </button>
        </div>
      </section>

      <section
        v-if="activeTab === 'mine' && mineOverview"
        class="carpool-overview"
      >
        <UiMetricStrip :style="{ '--metric-columns': 4 }">
          <UiMetric :label="t('carpool.totalPools')" :value="String(carpoolStats.total)" />
          <UiMetric :label="t('carpool.myOwnedPools')" :value="String(carpoolStats.owned)" />
          <UiMetric :label="t('carpool.joinedPools')" :value="String(carpoolStats.joined)" />
          <UiMetric :label="t('carpool.pendingApplications')" :value="String(carpoolStats.pending)" />
        </UiMetricStrip>

        <div class="mt-4 grid grid-cols-1 gap-3 md:grid-cols-[minmax(0,1fr)_220px]">
          <label class="min-w-0">
            <span class="sr-only">{{ t('carpool.searchPools') }}</span>
            <input
              v-model.trim="carpoolSearch"
              class="input"
              type="search"
              :placeholder="t('carpool.searchPoolsPlaceholder')"
            />
          </label>
          <label class="min-w-0">
            <span class="sr-only">{{ t('carpool.filterStatus') }}</span>
            <select v-model="carpoolStatusFilter" class="input">
              <option v-for="option in statusFilterOptions" :key="option.value" :value="option.value">
                {{ option.label }}
              </option>
            </select>
          </label>
        </div>
      </section>

      <section v-if="loading && !hasLoadedOnce" class="carpool-empty">
        {{ t('common.loading') }}
      </section>

      <template v-else>
        <template v-if="activeTab === 'mine'">
          <section class="space-y-4">
            <div class="flex items-center justify-between">
              <h2 class="text-base font-semibold text-[#202123] dark:text-[#ececf1]">
                {{ t('carpool.myOwnedPools') }}
              </h2>
              <span class="text-xs text-[#6e6e80] dark:text-[#acacbe]">
                {{ filteredOwnedPools.length }}/{{ ownedPools.length }}
              </span>
            </div>

            <div v-if="ownedPools.length === 0" class="carpool-empty">
              {{ t('carpool.emptyOwned') }}
            </div>
            <div v-else-if="filteredOwnedPools.length === 0" class="carpool-empty">
              {{ t('carpool.emptyFiltered') }}
            </div>
            <div v-else class="grid grid-cols-1 gap-4 xl:grid-cols-2">
              <article
                v-for="summary in pagedOwnedPools"
                :key="`owned-${summary.pool.id}`"
                class="carpool-pool-card"
              >
                <div class="flex flex-col gap-4">
                  <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                    <div class="space-y-2">
                      <div class="flex flex-wrap items-center gap-2">
                        <h3 class="text-lg font-semibold text-[#202123] dark:text-[#ececf1]">
                          {{ summary.pool.name }}
                        </h3>
                        <span :class="poolStatusClass(summary.pool.status)" class="rounded-full px-2.5 py-1 text-xs font-medium">
                          {{ poolStatusLabel(summary.pool.status) }}
                        </span>
                        <span class="rounded-full bg-[#f3f3f6] px-2.5 py-1 text-xs font-medium text-[#565869] dark:bg-[#2f2f2f] dark:text-[#c5c5d2]">
                          {{ t('carpool.owner') }}
                        </span>
                      </div>
                      <div class="flex flex-wrap items-center gap-2">
                        <GroupBadge
                          :name="summary.group_name || summary.pool.name"
                          :platform="summary.pool.platform"
                          scope="public"
                          subscription-type="subscription"
                          :show-rate="false"
                        />
                        <span class="text-xs text-[#6e6e80] dark:text-[#acacbe]">
                          {{ visibilityLabel(summary.pool.visibility) }}
                        </span>
                      </div>
                    </div>

                    <div class="flex flex-wrap gap-2">
                      <button type="button" class="btn btn-secondary btn-sm" @click="openDetailDialog(summary.pool.id)">
                        {{ t('carpool.openDetail') }}
                      </button>
                      <button type="button" class="btn btn-primary btn-sm" @click="openBindDialog(summary)">
                        {{ t('carpool.bindAccounts') }}
                      </button>
                      <button type="button" class="btn btn-danger btn-sm" :disabled="deletingPoolId === summary.pool.id" @click="askDeletePool(summary)">
                        {{ t('carpool.deletePool') }}
                      </button>
                    </div>
                  </div>

                  <div class="grid grid-cols-2 gap-3 lg:grid-cols-4">
                    <div class="rounded-xl bg-[#f3f3f6] p-3 dark:bg-[#171717]">
                      <div class="text-xs text-[#6e6e80] dark:text-[#acacbe]">{{ t('carpool.seats') }}</div>
                      <div class="mt-1 text-sm font-semibold text-[#202123] dark:text-[#ececf1]">
                        {{ summary.active_members }}/{{ summary.pool.target_seats }}
                      </div>
                    </div>
                    <div class="rounded-xl bg-[#f3f3f6] p-3 dark:bg-[#171717]">
                      <div class="text-xs text-[#6e6e80] dark:text-[#acacbe]">{{ t('carpool.pendingApplications') }}</div>
                      <div class="mt-1 text-sm font-semibold text-[#202123] dark:text-[#ececf1]">
                        {{ summary.pending_applications }}
                      </div>
                    </div>
                    <div class="rounded-xl bg-[#f3f3f6] p-3 dark:bg-[#171717]">
                      <div class="text-xs text-[#6e6e80] dark:text-[#acacbe]">{{ t('carpool.boundAccounts') }}</div>
                      <div class="mt-1 text-sm font-semibold text-[#202123] dark:text-[#ececf1]">
                        {{ summary.bound_account_count }}
                      </div>
                    </div>
                    <div class="rounded-xl bg-[#f3f3f6] p-3 dark:bg-[#171717]">
                      <div class="text-xs text-[#6e6e80] dark:text-[#acacbe]">{{ t('carpool.durationDays') }}</div>
                      <div class="mt-1 text-sm font-semibold text-[#202123] dark:text-[#ececf1]">
                        {{ summary.pool.duration_days }}
                      </div>
                    </div>
                  </div>

                  <div v-if="poolDetail(summary.pool.id)" class="grid grid-cols-1 gap-3 md:grid-cols-2">
                    <div class="rounded-xl bg-[#f3f3f6] px-3 py-3 dark:bg-[#171717]">
                      <UsageProgressBar
                        label="5h"
                        :utilization="poolDetailUsageUtilization(poolDetail(summary.pool.id), '5h')"
                        :resets-at="poolDetailUsageResetAt(poolDetail(summary.pool.id), '5h')"
                        :show-now-when-idle="true"
                        :show-label="false"
                        color="indigo"
                      />
                    </div>
                    <div class="rounded-xl bg-[#f3f3f6] px-3 py-3 dark:bg-[#171717]">
                      <UsageProgressBar
                        label="7d"
                        :utilization="poolDetailUsageUtilization(poolDetail(summary.pool.id), '7d')"
                        :resets-at="poolDetailUsageResetAt(poolDetail(summary.pool.id), '7d')"
                        :show-now-when-idle="true"
                        :show-label="false"
                        color="emerald"
                      />
                    </div>
                  </div>

                  <div class="grid grid-cols-1 gap-3">
                    <div v-if="poolExpanded(summary.pool.id)" class="rounded-xl border border-[#d9d9e3] p-3 dark:border-[#3f3f46]">
                      <div class="text-xs text-[#6e6e80] dark:text-[#acacbe]">{{ t('carpool.feeSummary') }}</div>
                      <div class="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-sm text-[#202123] dark:text-[#ececf1]">
                        <span>{{ t('carpool.seatPrice') }}: {{ formatMoney(summary.pool.seat_price) }}</span>
                        <span>{{ t('carpool.extraFee') }}: {{ formatExtraFeeMoney(summary.pool.extra_fee) }}</span>
                      </div>
                      <p v-if="summary.pool.extra_fee_description" class="mt-2 text-xs text-[#6e6e80] dark:text-[#acacbe]">
                        {{ summary.pool.extra_fee_description }}
                      </p>
                      <div class="mt-3 flex flex-wrap gap-1.5">
                        <span :class="serviceStatusBadgeClass(summary.pool.system_proxy_enabled)">
                          {{ t('carpool.systemProxyService') }} · {{ serviceStatusLabel(summary.pool.system_proxy_enabled) }}
                        </span>
                        <span :class="serviceStatusBadgeClass(summary.pool.risk_control_enabled)">
                          {{ t('carpool.riskControlService') }} · {{ serviceStatusLabel(summary.pool.risk_control_enabled) }}
                        </span>
                      </div>
                    </div>
                  </div>

                  <div v-if="poolExpanded(summary.pool.id) && poolDetailLoading(summary.pool.id)" class="rounded-xl border border-dashed border-[#c5c5d2] px-4 py-6 text-center text-sm text-[#6e6e80] dark:border-[#3f3f46] dark:text-[#acacbe]">
                    {{ t('common.loading') }}
                  </div>
                  <template v-else-if="poolExpanded(summary.pool.id) && poolDetail(summary.pool.id)">
                    <section class="grid grid-cols-1 gap-3">
                      <div class="rounded-xl border border-[#d9d9e3] p-3 dark:border-[#3f3f46]">
                        <div class="flex flex-wrap items-center justify-between gap-2">
                          <div class="text-xs text-[#6e6e80] dark:text-[#acacbe]">{{ t('carpool.inviteCode') }}</div>
                          <div class="flex flex-wrap gap-2">
                            <button type="button" class="btn btn-secondary btn-sm" @click="copyInviteCode(poolDetail(summary.pool.id)!.pool.invite_code)">
                              <Icon name="copy" size="sm" class="mr-1" />
                              {{ t('carpool.copyInviteCode') }}
                            </button>
                            <button type="button" class="btn btn-secondary btn-sm" @click="copyInviteLink(poolDetail(summary.pool.id)!.pool.invite_code)">
                              <Icon name="copy" size="sm" class="mr-1" />
                              {{ t('carpool.copyInviteLink') }}
                            </button>
                          </div>
                        </div>
                        <div class="mt-2 break-all rounded-lg bg-[#f3f3f6] px-3 py-2 font-mono text-sm text-[#202123] dark:bg-[#2f2f2f] dark:text-[#ececf1]">
                          {{ poolDetail(summary.pool.id)!.pool.invite_code }}
                        </div>
                      </div>

                      <div class="text-xs font-medium text-[#6e6e80] dark:text-[#acacbe]">
                        {{ t('carpool.accountWindowUsage') }}
                      </div>
                      <div class="grid grid-cols-1 gap-3 md:grid-cols-2">
                        <div class="rounded-xl bg-[#f3f3f6] px-3 py-3 dark:bg-[#171717]">
                          <UsageProgressBar
                            label="5h"
                            :utilization="poolDetailUsageUtilization(poolDetail(summary.pool.id), '5h')"
                            :resets-at="poolDetailUsageResetAt(poolDetail(summary.pool.id), '5h')"
                            :show-now-when-idle="true"
                            :show-label="false"
                            color="indigo"
                          />
                        </div>
                        <div class="rounded-xl bg-[#f3f3f6] px-3 py-3 dark:bg-[#171717]">
                          <UsageProgressBar
                            label="7d"
                            :utilization="poolDetailUsageUtilization(poolDetail(summary.pool.id), '7d')"
                            :resets-at="poolDetailUsageResetAt(poolDetail(summary.pool.id), '7d')"
                            :show-now-when-idle="true"
                            :show-label="false"
                            color="emerald"
                          />
                        </div>
                      </div>

                      <div class="rounded-xl border border-[#d9d9e3] p-3 dark:border-[#3f3f46]">
                        <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
                          <div class="text-xs font-medium text-[#6e6e80] dark:text-[#acacbe]">{{ t('carpool.accounts') }}</div>
                          <span class="text-xs text-[#6e6e80] dark:text-[#acacbe]">{{ poolDetail(summary.pool.id)!.accounts.length }}</span>
                        </div>
                        <div v-if="poolDetail(summary.pool.id)!.accounts.length === 0" class="rounded-lg border border-dashed border-[#c5c5d2] px-3 py-5 text-center text-xs text-[#6e6e80] dark:border-[#3f3f46] dark:text-[#acacbe]">
                          {{ t('carpool.noAccountsBound') }}
                        </div>
                        <div v-else class="grid grid-cols-1 gap-2">
                          <div
                            v-for="account in poolDetail(summary.pool.id)!.accounts"
                            :key="account.id"
                            class="rounded-lg bg-[#f3f3f6] p-3 dark:bg-[#171717]"
                          >
                            <div class="flex flex-wrap items-center justify-between gap-2">
                              <div class="min-w-0 truncate text-sm font-semibold text-[#202123] dark:text-[#ececf1]">
                                {{ account.name }}
                              </div>
                              <div class="flex flex-wrap items-center gap-2">
                                <span :class="accountStatusBadgeClass(account.status)">
                                  {{ accountStatusLabel(account.status) }}
                                </span>
                                <button
                                  type="button"
                                  class="btn btn-secondary btn-sm"
                                  :disabled="resettingAccountId === account.account_id"
                                  @click="askResetAccountLocalLimit(summary.pool.id, account)"
                                >
                                  <Icon name="refresh" size="sm" :class="resettingAccountId === account.account_id ? 'animate-spin' : ''" class="mr-1" />
                                  {{ t('carpool.resetLocalLimit') }}
                                </button>
                              </div>
                            </div>
                            <div class="mt-2 flex flex-wrap items-center gap-2">
                              <PlatformTypeBadge
                                :platform="account.platform"
                                :type="account.type"
                                :plan-type="account.account_level && account.account_level !== 'unknown' ? account.account_level : undefined"
                              />
                            </div>
                          </div>
                        </div>
                      </div>

                      <div class="rounded-xl border border-[#d9d9e3] p-3 dark:border-[#3f3f46]">
                        <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
                          <div class="text-xs font-medium text-[#6e6e80] dark:text-[#acacbe]">{{ t('carpool.members') }}</div>
                          <button
                            type="button"
                            class="btn btn-secondary btn-sm"
                            @click="openAllocationDialog(poolDetail(summary.pool.id)!)"
                          >
                            {{ t('carpool.editAllocation') }}
                          </button>
                        </div>
                        <div class="grid grid-cols-1 gap-2">
                          <div
                            v-for="member in poolVisibleMembers(poolDetail(summary.pool.id))"
                            :key="member.member.id"
                            class="rounded-lg bg-[#f3f3f6] p-3 dark:bg-[#171717]"
                          >
                            <div class="flex flex-wrap items-center justify-between gap-2">
                              <div class="min-w-0">
                                <div class="truncate text-sm font-semibold text-[#202123] dark:text-[#ececf1]">
                                  {{ member.username || member.masked_email }}
                                </div>
                                <div class="text-xs text-[#6e6e80] dark:text-[#acacbe]">
                                  {{ memberRoleLabel(member.member.role) }} · {{ formatPercent(memberAllocationPercent(member, poolDetail(summary.pool.id)?.pool.target_seats)) }}
                                </div>
                              </div>
                              <button
                                v-if="member.member.role !== 'owner' && member.member.status === 'active'"
                                type="button"
                                class="btn btn-danger btn-sm"
                                :disabled="actingMemberId === member.member.id"
                                @click="handleRemoveMember(member.member.id, summary.pool.id)"
                              >
                                {{ t('carpool.removeMember') }}
                              </button>
                            </div>
                            <div class="mt-2 space-y-1.5">
                              <UsageProgressBar
                                label="5h"
                                :utilization="memberUsageWindowUtilization(member, '5h')"
                                :resets-at="memberUsageWindowResetAt(member, '5h')"
                                :show-now-when-idle="true"
                                :show-label="false"
                                color="indigo"
                              />
                              <UsageProgressBar
                                label="7d"
                                :utilization="memberUsageWindowUtilization(member, '7d')"
                                :resets-at="memberUsageWindowResetAt(member, '7d')"
                                :show-now-when-idle="true"
                                :show-label="false"
                                color="emerald"
                              />
                            </div>
                          </div>
                        </div>
                      </div>

                      <div v-if="poolDetail(summary.pool.id)!.join_requests.length > 0" class="rounded-xl border border-[#d9d9e3] p-3 dark:border-[#3f3f46]">
                        <div class="mb-3 text-xs font-medium text-[#6e6e80] dark:text-[#acacbe]">{{ t('carpool.joinRequests') }}</div>
                        <div class="space-y-2">
                          <div
                            v-for="requestProfile in poolDetail(summary.pool.id)!.join_requests"
                            :key="requestProfile.request.id"
                            class="rounded-lg bg-[#f3f3f6] p-3 dark:bg-[#171717]"
                          >
                            <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                              <div class="min-w-0 space-y-2">
                                <div class="flex flex-wrap items-center gap-2">
                                  <span class="truncate text-sm font-semibold text-[#202123] dark:text-[#ececf1]">
                                    {{ requestProfile.username || requestProfile.masked_email }}
                                  </span>
                                  <span :class="requestStatusClass(requestProfile.request.status)" class="rounded-full px-2 py-0.5 text-[11px] font-medium">
                                    {{ requestStatusLabel(requestProfile.request.status) }}
                                  </span>
                                </div>
                                <div class="text-xs text-[#6e6e80] dark:text-[#acacbe]">{{ requestProfile.masked_email }}</div>
                                <div class="grid grid-cols-2 gap-2 text-xs text-[#6e6e80] dark:text-[#acacbe] md:grid-cols-4">
                                  <div>{{ t('carpool.totalRequests') }}: {{ formatInteger(requestProfile.usage.total_requests) }}</div>
                                  <div>{{ t('carpool.totalTokens') }}: {{ formatTokenCount(requestProfile.usage.total_tokens) }}</div>
                                  <div>{{ t('carpool.last7dTokens') }}: {{ formatTokenCount(requestProfile.usage.last_7d_tokens) }}</div>
                                  <div>{{ t('carpool.last30dTokens') }}: {{ formatTokenCount(requestProfile.usage.last_30d_tokens) }}</div>
                                </div>
                                <input
                                  v-model.trim="reviewNotes[requestProfile.request.id]"
                                  class="input"
                                  type="text"
                                  maxlength="200"
                                  :placeholder="t('carpool.reviewNotePlaceholder')"
                                />
                              </div>
                              <div class="flex flex-wrap gap-2">
                                <button
                                  v-if="requestProfile.request.status === 'pending'"
                                  type="button"
                                  class="btn btn-primary btn-sm"
                                  :disabled="actingRequestId === requestProfile.request.id"
                                  @click="handleApprove(requestProfile.request.id, summary.pool.id)"
                                >
                                  {{ t('carpool.approve') }}
                                </button>
                                <button
                                  v-if="requestProfile.request.status === 'pending'"
                                  type="button"
                                  class="btn btn-secondary btn-sm"
                                  :disabled="actingRequestId === requestProfile.request.id"
                                  @click="handleReject(requestProfile.request.id, summary.pool.id)"
                                >
                                  {{ t('carpool.reject') }}
                                </button>
                                <button
                                  v-if="requestProfile.request.status === 'approved'"
                                  type="button"
                                  class="btn btn-primary btn-sm"
                                  :disabled="actingRequestId === requestProfile.request.id"
                                  @click="handleConfirmPaid(requestProfile.request.id, summary.pool.id)"
                                >
                                  {{ t('carpool.confirmPaid') }}
                                </button>
                              </div>
                            </div>
                          </div>
                        </div>
                      </div>
                    </section>
                  </template>
                </div>
              </article>
            </div>

            <div v-if="ownedPageCount > 1" class="flex flex-col gap-3 rounded-2xl border border-[#d9d9e3] bg-white/80 p-3 dark:border-[#3f3f46] dark:bg-[#171717] sm:flex-row sm:items-center sm:justify-between">
              <div class="text-xs text-[#6e6e80] dark:text-[#acacbe]">
                {{ paginationInfo(ownedPage, filteredOwnedPools.length) }}
              </div>
              <div class="flex items-center justify-end gap-2">
                <button type="button" class="btn btn-secondary btn-sm" :disabled="ownedPage <= 1" @click="setOwnedPage(ownedPage - 1)">
                  {{ t('carpool.previousPage') }}
                </button>
                <span class="min-w-16 text-center text-xs font-medium text-[#6e6e80] dark:text-[#acacbe]">
                  {{ ownedPage }}/{{ ownedPageCount }}
                </span>
                <button type="button" class="btn btn-secondary btn-sm" :disabled="ownedPage >= ownedPageCount" @click="setOwnedPage(ownedPage + 1)">
                  {{ t('carpool.nextPage') }}
                </button>
              </div>
            </div>
          </section>

          <section class="space-y-4">
            <div class="flex items-center justify-between">
              <h2 class="text-base font-semibold text-[#202123] dark:text-[#ececf1]">
                {{ t('carpool.joinedPools') }}
              </h2>
              <span class="text-xs text-[#6e6e80] dark:text-[#acacbe]">
                {{ filteredJoinedPools.length }}/{{ joinedPools.length }}
              </span>
            </div>

            <div v-if="joinedPools.length === 0" class="carpool-empty">
              {{ t('carpool.emptyJoined') }}
            </div>
            <div v-else-if="filteredJoinedPools.length === 0" class="carpool-empty">
              {{ t('carpool.emptyFiltered') }}
            </div>
            <div v-else class="grid grid-cols-1 gap-4 xl:grid-cols-2">
              <article
                v-for="summary in pagedJoinedPools"
                :key="`joined-${summary.pool.id}`"
                class="carpool-pool-card"
              >
                <div class="flex flex-col gap-4">
                  <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                    <div class="space-y-2">
                      <div class="flex flex-wrap items-center gap-2">
                        <h3 class="text-lg font-semibold text-[#202123] dark:text-[#ececf1]">
                          {{ summary.pool.name }}
                        </h3>
                        <span :class="poolStatusClass(summary.pool.status)" class="rounded-full px-2.5 py-1 text-xs font-medium">
                          {{ poolStatusLabel(summary.pool.status) }}
                        </span>
                        <span class="rounded-full bg-[#f3f3f6] px-2.5 py-1 text-xs font-medium text-[#565869] dark:bg-[#2f2f2f] dark:text-[#c5c5d2]">
                          {{ currentUserStatusLabel(summary.current_user_status) }}
                        </span>
                      </div>
                      <GroupBadge
                        :name="summary.group_name || summary.pool.name"
                        :platform="summary.pool.platform"
                        scope="public"
                        subscription-type="subscription"
                        :show-rate="false"
                      />
                    </div>

                    <div class="flex flex-wrap gap-2">
                      <button type="button" class="btn btn-secondary btn-sm" @click="openDetailDialog(summary.pool.id)">
                        {{ t('carpool.openDetail') }}
                      </button>
                      <button type="button" class="btn btn-secondary btn-sm" :disabled="poolDetailLoading(summary.pool.id)" @click="loadPoolDetail(summary.pool.id)">
                        <Icon name="refresh" size="sm" :class="poolDetailLoading(summary.pool.id) ? 'animate-spin' : ''" class="mr-1.5" />
                        {{ t('common.refresh') }}
                      </button>
                    </div>
                  </div>

                  <div class="grid grid-cols-2 gap-3 lg:grid-cols-3">
                    <div class="rounded-xl bg-[#f3f3f6] p-3 dark:bg-[#171717]">
                      <div class="text-xs text-[#6e6e80] dark:text-[#acacbe]">{{ t('carpool.seats') }}</div>
                      <div class="mt-1 text-sm font-semibold text-[#202123] dark:text-[#ececf1]">
                        {{ summary.active_members }}/{{ summary.pool.target_seats }}
                      </div>
                    </div>
                    <div class="rounded-xl bg-[#f3f3f6] p-3 dark:bg-[#171717]">
                      <div class="text-xs text-[#6e6e80] dark:text-[#acacbe]">{{ t('carpool.durationDays') }}</div>
                      <div class="mt-1 text-sm font-semibold text-[#202123] dark:text-[#ececf1]">
                        {{ summary.pool.duration_days }}
                      </div>
                    </div>
                    <div class="rounded-xl bg-[#f3f3f6] p-3 dark:bg-[#171717]">
                      <div class="text-xs text-[#6e6e80] dark:text-[#acacbe]">{{ t('carpool.seatPrice') }}</div>
                      <div class="mt-1 text-sm font-semibold text-[#202123] dark:text-[#ececf1]">
                        {{ formatMoney(summary.pool.seat_price) }}
                      </div>
                    </div>
                  </div>

                  <div v-if="poolDetail(summary.pool.id)" class="grid grid-cols-1 gap-3 md:grid-cols-2">
                    <div class="rounded-xl bg-[#f3f3f6] px-3 py-3 dark:bg-[#171717]">
                      <UsageProgressBar
                        label="5h"
                        :utilization="currentMemberUsageUtilization(poolDetail(summary.pool.id), '5h')"
                        :resets-at="currentMemberUsageResetAt(poolDetail(summary.pool.id), '5h')"
                        :show-now-when-idle="true"
                        :show-label="false"
                        color="indigo"
                      />
                    </div>
                    <div class="rounded-xl bg-[#f3f3f6] px-3 py-3 dark:bg-[#171717]">
                      <UsageProgressBar
                        label="7d"
                        :utilization="currentMemberUsageUtilization(poolDetail(summary.pool.id), '7d')"
                        :resets-at="currentMemberUsageResetAt(poolDetail(summary.pool.id), '7d')"
                        :show-now-when-idle="true"
                        :show-label="false"
                        color="emerald"
                      />
                    </div>
                  </div>

                  <div v-if="poolExpanded(summary.pool.id) && poolDetailLoading(summary.pool.id)" class="rounded-xl border border-dashed border-[#c5c5d2] px-4 py-6 text-center text-sm text-[#6e6e80] dark:border-[#3f3f46] dark:text-[#acacbe]">
                    {{ t('common.loading') }}
                  </div>
                  <template v-else-if="poolExpanded(summary.pool.id) && poolDetail(summary.pool.id)">
                    <div class="rounded-xl border border-[#d9d9e3] p-3 dark:border-[#3f3f46]">
                      <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
                        <div class="text-xs font-medium text-[#6e6e80] dark:text-[#acacbe]">{{ t('carpool.accounts') }}</div>
                        <span class="text-xs text-[#6e6e80] dark:text-[#acacbe]">{{ poolDetail(summary.pool.id)!.accounts.length }}</span>
                      </div>
                      <div v-if="poolDetail(summary.pool.id)!.accounts.length === 0" class="rounded-lg border border-dashed border-[#c5c5d2] px-3 py-5 text-center text-xs text-[#6e6e80] dark:border-[#3f3f46] dark:text-[#acacbe]">
                        {{ t('carpool.noAccountsBound') }}
                      </div>
                      <div v-else class="grid grid-cols-1 gap-2">
                        <div
                          v-for="account in poolDetail(summary.pool.id)!.accounts"
                          :key="account.id"
                          class="rounded-lg bg-[#f3f3f6] p-3 dark:bg-[#171717]"
                        >
                          <div class="flex flex-wrap items-center justify-between gap-2">
                            <div class="min-w-0 truncate text-sm font-semibold text-[#202123] dark:text-[#ececf1]">
                              {{ account.name }}
                            </div>
                            <span :class="accountStatusBadgeClass(account.status)">
                              {{ accountStatusLabel(account.status) }}
                            </span>
                          </div>
                          <div class="mt-2 flex flex-wrap items-center gap-2">
                            <PlatformTypeBadge
                              :platform="account.platform"
                              :type="account.type"
                              :plan-type="account.account_level && account.account_level !== 'unknown' ? account.account_level : undefined"
                            />
                          </div>
                        </div>
                      </div>
                    </div>
                    <div class="rounded-xl border border-[#d9d9e3] p-3 dark:border-[#3f3f46]">
                      <div class="mb-3 text-xs font-medium text-[#6e6e80] dark:text-[#acacbe]">{{ t('carpool.members') }}</div>
                      <div class="grid grid-cols-1 gap-2">
                        <div
                          v-for="member in poolVisibleMembers(poolDetail(summary.pool.id))"
                          :key="member.member.id"
                          class="rounded-lg bg-[#f3f3f6] p-3 dark:bg-[#171717]"
                        >
                          <div class="flex flex-wrap items-center justify-between gap-2">
                            <div class="min-w-0">
                              <div class="truncate text-sm font-semibold text-[#202123] dark:text-[#ececf1]">
                                {{ member.username || member.masked_email }}
                              </div>
                              <div class="text-xs text-[#6e6e80] dark:text-[#acacbe]">
                                {{ memberRoleLabel(member.member.role) }} · {{ formatPercent(memberAllocationPercent(member, poolDetail(summary.pool.id)?.pool.target_seats)) }}
                              </div>
                            </div>
                            <div class="text-right text-xs text-[#6e6e80] dark:text-[#acacbe]">
                              <div>{{ t('carpool.totalTokens') }}: {{ formatTokenCount(member.total_tokens) }}</div>
                              <div>{{ t('carpool.totalCost') }}: {{ formatMoney(member.total_cost_usd) }}</div>
                            </div>
                          </div>
                          <div class="mt-2 space-y-1.5">
                            <UsageProgressBar
                              label="5h"
                              :utilization="memberUsageWindowUtilization(member, '5h')"
                              :resets-at="memberUsageWindowResetAt(member, '5h')"
                              :show-now-when-idle="true"
                              :show-label="false"
                              color="indigo"
                            />
                            <UsageProgressBar
                              label="7d"
                              :utilization="memberUsageWindowUtilization(member, '7d')"
                              :resets-at="memberUsageWindowResetAt(member, '7d')"
                              :show-now-when-idle="true"
                              :show-label="false"
                              color="emerald"
                            />
                          </div>
                        </div>
                      </div>
                    </div>
                  </template>
                </div>
              </article>
            </div>

            <div v-if="joinedPageCount > 1" class="flex flex-col gap-3 rounded-2xl border border-[#d9d9e3] bg-white/80 p-3 dark:border-[#3f3f46] dark:bg-[#171717] sm:flex-row sm:items-center sm:justify-between">
              <div class="text-xs text-[#6e6e80] dark:text-[#acacbe]">
                {{ paginationInfo(joinedPage, filteredJoinedPools.length) }}
              </div>
              <div class="flex items-center justify-end gap-2">
                <button type="button" class="btn btn-secondary btn-sm" :disabled="joinedPage <= 1" @click="setJoinedPage(joinedPage - 1)">
                  {{ t('carpool.previousPage') }}
                </button>
                <span class="min-w-16 text-center text-xs font-medium text-[#6e6e80] dark:text-[#acacbe]">
                  {{ joinedPage }}/{{ joinedPageCount }}
                </span>
                <button type="button" class="btn btn-secondary btn-sm" :disabled="joinedPage >= joinedPageCount" @click="setJoinedPage(joinedPage + 1)">
                  {{ t('carpool.nextPage') }}
                </button>
              </div>
            </div>
          </section>
        </template>

        <template v-else>
          <section class="rounded-2xl border border-[#d9d9e3] bg-[#f7f7f8] p-5 shadow-sm dark:border-[#3f3f46] dark:bg-[#171717] sm:p-6">
            <div class="grid grid-cols-1 gap-5 lg:grid-cols-[minmax(0,1fr)_minmax(320px,480px)] lg:items-center">
              <div class="min-w-0 space-y-3">
                <div class="inline-flex items-center gap-2 rounded-full border border-[#d9d9e3] bg-white/80 px-3 py-1 text-xs font-medium text-[#565869] dark:border-[#3f3f46] dark:bg-[#2f2f2f] dark:text-[#c5c5d2]">
                  <Icon name="key" size="sm" />
                  <span>{{ t('carpool.joinByInvite') }}</span>
                </div>
                <div>
                  <h2 class="text-xl font-semibold tracking-tight text-[#202123] dark:text-[#ececf1]">
                    {{ t('carpool.inviteEntryTitle') }}
                  </h2>
                  <p class="mt-2 max-w-2xl text-sm leading-6 text-[#6e6e80] dark:text-[#acacbe]">
                    {{ t('carpool.joinByInviteHint') }}
                  </p>
                </div>
              </div>

              <div class="rounded-2xl border border-[#d9d9e3] bg-white/95 p-4 shadow-sm dark:border-[#3f3f46] dark:bg-[#171717]">
                <label class="input-label">{{ t('carpool.inviteCode') }}</label>
                <div class="mt-2 flex flex-col gap-2 sm:flex-row">
                  <input
                    v-model.trim="inviteInput"
                    class="input min-w-0 flex-1"
                    type="text"
                    :placeholder="t('carpool.inviteInputPlaceholder')"
                    @keyup.enter="resolveInviteAndOpenApply"
                  />
                  <button type="button" class="btn btn-primary flex-shrink-0" :disabled="resolvingInvite || !inviteInput.trim()" @click="resolveInviteAndOpenApply">
                    {{ resolvingInvite ? t('common.loading') : t('carpool.resolveInvite') }}
                  </button>
                </div>
              </div>
            </div>
          </section>
        </template>
      </template>
    </UiPage>

    <BaseDialog
      :show="showCreateDialog"
      :title="t('carpool.createDialogTitle')"
      width="wide"
      @close="closeCreateDialog"
    >
      <form class="grid grid-cols-1 gap-4 md:grid-cols-2" @submit.prevent="submitCreate">
        <div class="md:col-span-2">
          <label class="input-label">{{ t('carpool.name') }}</label>
          <input v-model.trim="createForm.name" class="input" type="text" maxlength="120" :placeholder="t('carpool.namePlaceholder')" required />
        </div>
        <div>
          <label class="input-label">{{ t('carpool.platform') }}</label>
          <select v-model="createForm.platform" class="input">
            <option v-for="option in platformOptions" :key="option.value" :value="option.value">
              {{ option.label }}
            </option>
          </select>
        </div>
        <div>
          <label class="input-label">{{ t('carpool.targetSeats') }}</label>
          <select v-model.number="createForm.targetSeats" class="input">
            <option v-for="seat in seatOptions" :key="seat" :value="seat">
              {{ seat }}
            </option>
          </select>
        </div>
        <div>
          <label class="input-label">{{ t('carpool.durationDays') }}</label>
          <input v-model.number="createForm.durationDays" class="input" type="number" min="1" max="365" required />
        </div>
        <div>
          <label class="input-label">{{ t('carpool.seatPrice') }}</label>
          <input v-model.number="createForm.seatPrice" class="input" type="number" min="0" step="0.01" required />
        </div>
        <section class="md:col-span-2 rounded-2xl border border-[#d9d9e3] bg-[#f7f7f8] p-4 dark:border-[#3f3f46] dark:bg-[#171717]">
          <div class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
            <div>
              <div class="text-sm font-semibold text-[#202123] dark:text-[#ececf1]">
                {{ t('carpool.ownerPaidServices') }}
              </div>
              <p class="mt-1 text-xs leading-5 text-[#6e6e80] dark:text-[#acacbe]">
                {{ t('carpool.ownerPaidServicesHint') }}
              </p>
              <p class="mt-1 text-xs leading-5 font-medium text-[#565869] dark:text-[#c5c5d2]">
                {{ t('carpool.defaultServiceFeeHint', { amount: formatExtraFeeMoney(carpoolBaseServiceFee) }) }}
              </p>
            </div>
            <div class="rounded-full bg-[#171717] px-3 py-1 text-xs font-semibold text-white dark:bg-[#ececf1] dark:text-[#171717]">
              {{ t('carpool.selectedExtraFeeSummary', { amount: formatExtraFeeMoney(createExtraFee) }) }}
            </div>
          </div>

          <div class="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-2">
            <button
              type="button"
              class="flex cursor-pointer gap-3 rounded-xl border p-4 text-left transition-colors"
              :class="createForm.useSystemProxyService
                ? 'border-[#171717] bg-white shadow-sm dark:border-[#ececf1] dark:bg-[#171717]'
                : 'border-[#d9d9e3] bg-white/90 hover:border-[#d8c2a8] dark:border-[#3f3f46] dark:bg-[#171717] dark:hover:border-[#5a4635]'"
              @click="createForm.useSystemProxyService = !createForm.useSystemProxyService"
            >
              <span
                class="mt-0.5 flex h-6 w-11 flex-shrink-0 items-center rounded-full p-0.5 transition-colors"
                :class="createForm.useSystemProxyService ? 'bg-[#171717] dark:bg-[#ececf1]' : 'bg-[#d8c9b8] dark:bg-[#3f3f46]'"
              >
                <span
                  class="h-5 w-5 rounded-full bg-white shadow-sm transition-transform dark:bg-[#171717]"
                  :class="createForm.useSystemProxyService ? 'translate-x-5 dark:bg-[#171717]' : 'translate-x-0'"
                ></span>
              </span>
              <span class="min-w-0 flex-1">
                <span class="flex flex-wrap items-center gap-2">
                  <span class="text-sm font-semibold text-[#202123] dark:text-[#ececf1]">{{ t('carpool.systemProxyService') }}</span>
                  <span class="rounded-full bg-[#e6f6f1] px-2 py-0.5 text-[11px] font-semibold text-[#765f48] dark:bg-[#2f2f2f] dark:text-[#dcc7ad]">
                    {{ t('carpool.monthlyFee', { amount: formatExtraFeeMoney(carpoolSystemProxyFee) }) }}
                  </span>
                </span>
                <span class="mt-1 block text-xs leading-5 text-[#6e6e80] dark:text-[#acacbe]">
                  {{ t('carpool.systemProxyServiceDescription') }}
                </span>
              </span>
            </button>

            <button
              type="button"
              class="flex cursor-pointer gap-3 rounded-xl border p-4 text-left transition-colors"
              :class="createForm.useRiskControlService
                ? 'border-[#171717] bg-white shadow-sm dark:border-[#ececf1] dark:bg-[#171717]'
                : 'border-[#d9d9e3] bg-white/90 hover:border-[#d8c2a8] dark:border-[#3f3f46] dark:bg-[#171717] dark:hover:border-[#5a4635]'"
              @click="createForm.useRiskControlService = !createForm.useRiskControlService"
            >
              <span
                class="mt-0.5 flex h-6 w-11 flex-shrink-0 items-center rounded-full p-0.5 transition-colors"
                :class="createForm.useRiskControlService ? 'bg-[#171717] dark:bg-[#ececf1]' : 'bg-[#d8c9b8] dark:bg-[#3f3f46]'"
              >
                <span
                  class="h-5 w-5 rounded-full bg-white shadow-sm transition-transform dark:bg-[#171717]"
                  :class="createForm.useRiskControlService ? 'translate-x-5 dark:bg-[#171717]' : 'translate-x-0'"
                ></span>
              </span>
              <span class="min-w-0 flex-1">
                <span class="flex flex-wrap items-center gap-2">
                  <span class="text-sm font-semibold text-[#202123] dark:text-[#ececf1]">{{ t('carpool.riskControlService') }}</span>
                  <span class="rounded-full bg-[#e6f6f1] px-2 py-0.5 text-[11px] font-semibold text-[#765f48] dark:bg-[#2f2f2f] dark:text-[#dcc7ad]">
                    {{ t('carpool.monthlyFee', { amount: formatExtraFeeMoney(carpoolRiskControlFee) }) }}
                  </span>
                </span>
                <span class="mt-1 block text-xs leading-5 text-[#6e6e80] dark:text-[#acacbe]">
                  {{ t('carpool.riskControlServiceDescription') }}
                </span>
              </span>
            </button>
          </div>

          <div class="mt-3 rounded-xl border border-[#d9d9e3] bg-white/70 px-3 py-2 text-xs text-[#6e6e80] dark:border-[#3f3f46] dark:bg-[#171717] dark:text-[#acacbe]">
            <span class="font-medium text-[#202123] dark:text-[#ececf1]">{{ t('carpool.extraFeeDescription') }}:</span>
            {{ createExtraFeeDescription || t('carpool.noExtraFeeSelected') }}
          </div>
        </section>
        <div class="md:col-span-2">
          <label class="input-label">{{ t('carpool.notes') }}</label>
          <textarea
            v-model.trim="createForm.notes"
            class="input min-h-[112px] resize-y"
            maxlength="1000"
            :placeholder="t('carpool.notePlaceholder')"
          />
        </div>
      </form>

      <template #footer>
        <div class="flex flex-wrap justify-end gap-3">
          <button type="button" class="btn btn-secondary" :disabled="creatingPool" @click="closeCreateDialog">
            {{ t('common.cancel') }}
          </button>
          <button type="button" class="btn btn-primary" :disabled="creatingPool" @click="submitCreate">
            {{ creatingPool ? t('common.loading') : t('common.create') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <BaseDialog
      :show="showBindDialog"
      :title="t('carpool.bindDialogTitle')"
      width="extra-wide"
      @close="closeBindDialog"
    >
      <div class="space-y-4">
        <div class="rounded-xl border border-dashed border-[#c5c5d2] bg-[#f7f7f8] px-4 py-3 text-sm text-[#565869] dark:border-[#3f3f46] dark:bg-[#1a1512] dark:text-[#c5c5d2]">
          {{ t('carpool.bindAccountsHint') }}
        </div>

        <div class="flex flex-wrap items-center justify-between gap-3">
          <div class="text-sm text-[#6e6e80] dark:text-[#acacbe]">
            {{ t('carpool.selectedAccounts', { count: selectedBindAccountIds.length }) }}
          </div>
          <button type="button" class="btn btn-secondary btn-sm" :disabled="loadingOwnerAccounts" @click="reloadOwnerAccounts">
            <Icon name="refresh" size="sm" :class="loadingOwnerAccounts ? 'animate-spin' : ''" class="mr-1.5" />
            {{ t('common.refresh') }}
          </button>
        </div>

        <div v-if="loadingOwnerAccounts" class="rounded-xl border border-[#d9d9e3] px-4 py-10 text-center text-sm text-[#6e6e80] dark:border-[#3f3f46] dark:text-[#acacbe]">
          {{ t('common.loading') }}
        </div>
        <div v-else-if="ownerAccounts.length === 0" class="rounded-xl border border-[#d9d9e3] px-4 py-10 text-center text-sm text-[#6e6e80] dark:border-[#3f3f46] dark:text-[#acacbe]">
          {{ t('carpool.noAccountsForPlatform') }}
        </div>
        <div v-else class="grid grid-cols-1 gap-3 lg:grid-cols-2">
          <label
            v-for="account in ownerAccounts"
            :key="account.id"
            class="flex cursor-pointer gap-3 rounded-2xl border border-[#d9d9e3] bg-white/95 p-4 transition-colors hover:border-[#d7bea1] dark:border-[#3f3f46] dark:bg-[#171717] dark:hover:border-[#5a4635]"
          >
            <input
              :checked="selectedBindAccountIds.includes(account.id)"
              class="mt-1 h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
              type="checkbox"
              @change="toggleBindAccount(account.id)"
            />
            <div class="min-w-0 flex-1 space-y-2">
              <div class="flex flex-wrap items-center gap-2">
                <div class="min-w-0 truncate text-sm font-semibold text-[#202123] dark:text-[#ececf1]">
                  {{ account.name }}
                </div>
                <span class="rounded-full bg-[#f3f3f6] px-2 py-0.5 text-[11px] font-medium text-[#565869] dark:bg-[#2f2f2f] dark:text-[#c5c5d2]">
                  {{ platformLabel(account.platform) }}
                </span>
              </div>
              <div class="grid grid-cols-2 gap-2 text-xs text-[#6e6e80] dark:text-[#acacbe]">
                <div>{{ t('carpool.fiveHourQuota') }}: {{ formatMoney(account.window_cost_limit ?? 0) }}</div>
                <div>{{ t('carpool.weeklyQuota') }}: {{ formatMoney(account.quota_weekly_limit ?? 0) }}</div>
                <div>{{ t('carpool.shareMode') }}: {{ account.share_mode || '-' }}</div>
                <div>{{ t('carpool.statusLabel') }}: {{ account.status }}</div>
              </div>
            </div>
          </label>
        </div>
      </div>

      <template #footer>
        <div class="flex flex-wrap justify-end gap-3">
          <button type="button" class="btn btn-secondary" :disabled="bindingAccounts" @click="closeBindDialog">
            {{ t('common.cancel') }}
          </button>
          <button type="button" class="btn btn-primary" :disabled="bindingAccounts || selectedBindAccountIds.length === 0" @click="submitBindAccounts">
            {{ bindingAccounts ? t('common.loading') : t('common.save') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <BaseDialog
      :show="showApplyDialog"
      :title="t('carpool.applyDialogTitle')"
      width="wide"
      @close="closeApplyDialog"
    >
      <div class="space-y-4">
        <div v-if="applyTarget" class="rounded-xl border border-[#d9d9e3] bg-[#f3f3f6] p-4 dark:border-[#3f3f46] dark:bg-[#171717]">
          <div class="text-base font-semibold text-[#202123] dark:text-[#ececf1]">
            {{ applyTarget.pool.name }}
          </div>
          <div class="mt-2 flex flex-wrap gap-3 text-sm text-[#6e6e80] dark:text-[#acacbe]">
            <span>{{ t('carpool.seats') }}: {{ applyTarget.active_members }}/{{ applyTarget.pool.target_seats }}</span>
            <span>{{ t('carpool.seatPrice') }}: {{ formatMoney(applyTarget.pool.seat_price) }}</span>
            <span>{{ t('carpool.extraFee') }}: {{ formatExtraFeeMoney(applyTarget.pool.extra_fee) }}</span>
          </div>
        </div>
        <div>
          <label class="input-label">{{ t('carpool.note') }}</label>
          <textarea
            v-model.trim="applyNote"
            class="input min-h-[112px] resize-y"
            maxlength="500"
            :placeholder="t('carpool.notePlaceholder')"
          />
        </div>
      </div>

      <template #footer>
        <div class="flex flex-wrap justify-end gap-3">
          <button type="button" class="btn btn-secondary" :disabled="submittingApply" @click="closeApplyDialog">
            {{ t('common.cancel') }}
          </button>
          <button type="button" class="btn btn-primary" :disabled="submittingApply || !applyTarget" @click="submitApply">
            {{ submittingApply ? t('common.loading') : t('common.submit') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <BaseDialog
      :show="showDetailDialog"
      :title="t('carpool.detailDialogTitle')"
      width="extra-wide"
      @close="closeDetailDialog"
    >
      <div v-if="detailLoading && !detailData" class="py-10 text-center text-sm text-[#6e6e80] dark:text-[#acacbe]">
        {{ t('common.loading') }}
      </div>

      <div v-else-if="detailData" class="space-y-6">
        <section class="rounded-2xl border border-[#d9d9e3] bg-white/95 p-5 dark:border-[#3f3f46] dark:bg-[#171717]">
          <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
            <div class="space-y-3">
              <div class="flex flex-wrap items-center gap-2">
                <h3 class="text-xl font-semibold text-[#202123] dark:text-[#ececf1]">
                  {{ detailData.pool.name }}
                </h3>
                <span :class="poolStatusClass(detailData.pool.status)" class="rounded-full px-2.5 py-1 text-xs font-medium">
                  {{ poolStatusLabel(detailData.pool.status) }}
                </span>
              </div>
              <div class="flex flex-wrap items-center gap-2">
                <GroupBadge
                  :name="detailData.group?.name || detailData.summary.group_name || detailData.pool.name"
                  :platform="detailData.pool.platform"
                  :scope="detailData.group?.scope || 'public'"
                  :subscription-type="detailData.group?.subscription_type || 'subscription'"
                  :show-rate="false"
                />
                <span class="text-xs text-[#6e6e80] dark:text-[#acacbe]">
                  {{ visibilityLabel(detailData.pool.visibility) }}
                </span>
              </div>
              <p v-if="detailData.pool.notes" class="text-sm text-[#6e6e80] dark:text-[#acacbe]">
                {{ detailData.pool.notes }}
              </p>
            </div>

            <div class="flex flex-wrap gap-2">
              <button
                v-if="detailData.summary.is_owner"
                type="button"
                class="btn btn-primary btn-sm"
                @click="openBindDialog(detailData.summary)"
              >
                {{ t('carpool.bindAccounts') }}
              </button>
              <button
                v-if="detailData.summary.is_owner"
                type="button"
                class="btn btn-danger btn-sm"
                :disabled="deletingPoolId === detailData.pool.id"
                @click="askDeletePool(detailData.summary)"
              >
                {{ t('carpool.deletePool') }}
              </button>
              <button type="button" class="btn btn-secondary btn-sm" :disabled="detailLoading" @click="reloadDetail">
                <Icon name="refresh" size="sm" :class="detailLoading ? 'animate-spin' : ''" class="mr-1.5" />
                {{ t('common.refresh') }}
              </button>
            </div>
          </div>

          <div
            v-if="detailData.summary.is_owner"
            class="mt-4 grid grid-cols-1 gap-4 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-start"
          >
            <div class="rounded-xl border border-[#d9d9e3] p-4 dark:border-[#3f3f46]">
              <div class="text-xs text-[#6e6e80] dark:text-[#acacbe]">{{ t('carpool.inviteCode') }}</div>
              <div class="mt-2 flex flex-wrap items-center gap-3">
                <code class="rounded-lg bg-[#f3f3f6] px-3 py-2 text-sm font-medium text-[#202123] dark:bg-[#2f2f2f] dark:text-[#ececf1]">
                  {{ detailData.pool.invite_code }}
                </code>
                <button type="button" class="btn btn-secondary btn-sm" @click="copyInviteCode(detailData.pool.invite_code)">
                  <Icon name="copy" size="sm" class="mr-1.5" />
                  {{ t('carpool.copyInviteCode') }}
                </button>
                <button type="button" class="btn btn-secondary btn-sm" @click="copyInviteLink(detailData.pool.invite_code)">
                  <Icon name="copy" size="sm" class="mr-1.5" />
                  {{ t('carpool.copyInviteLink') }}
                </button>
              </div>
              <p class="mt-2 break-all text-xs text-[#6e6e80] dark:text-[#acacbe]">
                {{ buildCarpoolInviteLink(detailData.pool.invite_code) }}
              </p>
            </div>
            <div class="rounded-xl border border-[#d9d9e3] p-4 dark:border-[#3f3f46]">
              <div class="text-xs text-[#6e6e80] dark:text-[#acacbe]">{{ t('carpool.feeSummary') }}</div>
              <div class="mt-2 space-y-1 text-sm text-[#202123] dark:text-[#ececf1]">
	                <div>{{ t('carpool.seatPrice') }}: {{ formatMoney(detailData.pool.seat_price) }}</div>
	                <div>{{ t('carpool.extraFee') }}: {{ formatExtraFeeMoney(detailData.pool.extra_fee) }}</div>
	                <div>{{ t('carpool.durationDays') }}: {{ detailData.pool.duration_days }}</div>
		                <div class="space-y-1 pt-1">
		                  <div class="text-xs text-[#6e6e80] dark:text-[#acacbe]">{{ t('carpool.serviceStatus') }}</div>
		                  <div class="flex flex-wrap gap-1.5">
		                    <span :class="serviceStatusBadgeClass(detailData.pool.system_proxy_enabled)">
		                      {{ t('carpool.systemProxyService') }} · {{ serviceStatusLabel(detailData.pool.system_proxy_enabled) }}
		                    </span>
		                    <span :class="serviceStatusBadgeClass(detailData.pool.risk_control_enabled)">
		                      {{ t('carpool.riskControlService') }} · {{ serviceStatusLabel(detailData.pool.risk_control_enabled) }}
		                    </span>
		                  </div>
		                </div>
	              </div>
	            </div>
          </div>
        </section>

        <section class="rounded-2xl border border-[#d9d9e3] bg-white/95 p-5 dark:border-[#3f3f46] dark:bg-[#171717]">
          <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
            <h4 class="text-base font-semibold text-[#202123] dark:text-[#ececf1]">
              {{ t('carpool.accountWindowUsage') }}
            </h4>
            <span class="text-xs text-[#6e6e80] dark:text-[#acacbe]">
              {{ t('carpool.boundAccounts') }}: {{ detailData.summary.bound_account_count }}
            </span>
          </div>
          <div class="grid grid-cols-1 gap-3 md:grid-cols-2">
            <div class="rounded-xl bg-[#f3f3f6] px-3 py-3 dark:bg-[#171717]">
              <UsageProgressBar
                :label="t('carpool.fiveHourUsage')"
                :utilization="poolUsageWindowUtilization('5h')"
                :resets-at="poolUsageWindowResetAt('5h')"
	                :show-now-when-idle="true"
                :show-label="false"
	                color="indigo"
	              />
	            </div>
            <div class="rounded-xl bg-[#f3f3f6] px-3 py-3 dark:bg-[#171717]">
              <UsageProgressBar
                :label="t('carpool.weeklyUsage')"
                :utilization="poolUsageWindowUtilization('7d')"
                :resets-at="poolUsageWindowResetAt('7d')"
	                :show-now-when-idle="true"
                :show-label="false"
	                color="emerald"
	              />
	            </div>
          </div>
        </section>

        <section class="rounded-2xl border border-[#d9d9e3] bg-white/95 p-5 dark:border-[#3f3f46] dark:bg-[#171717]">
          <div class="mb-4 flex items-center justify-between">
            <h4 class="text-base font-semibold text-[#202123] dark:text-[#ececf1]">
              {{ t('carpool.accounts') }}
            </h4>
            <span class="text-xs text-[#6e6e80] dark:text-[#acacbe]">{{ detailData.accounts.length }}</span>
          </div>
          <div v-if="detailData.accounts.length === 0" class="rounded-xl border border-dashed border-[#c5c5d2] px-4 py-8 text-center text-sm text-[#6e6e80] dark:border-[#3f3f46] dark:text-[#acacbe]">
            {{ t('carpool.noAccountsBound') }}
          </div>
          <div v-else class="grid grid-cols-1 gap-3 lg:grid-cols-2">
            <div
              v-for="account in detailData.accounts"
              :key="account.id"
              class="rounded-xl border border-[#d9d9e3] p-4 dark:border-[#3f3f46]"
            >
              <div class="flex flex-wrap items-center gap-2">
                <div class="text-sm font-semibold text-[#202123] dark:text-[#ececf1]">{{ account.name }}</div>
                <span :class="accountStatusBadgeClass(account.status)">
                  {{ accountStatusLabel(account.status) }}
                </span>
                <button
                  v-if="detailData.summary.is_owner"
                  type="button"
                  class="btn btn-secondary btn-sm"
                  :disabled="resettingAccountId === account.account_id"
                  @click="askResetAccountLocalLimit(detailData.pool.id, account)"
                >
                  <Icon name="refresh" size="sm" :class="resettingAccountId === account.account_id ? 'animate-spin' : ''" class="mr-1" />
                  {{ t('carpool.resetLocalLimit') }}
                </button>
              </div>
              <div class="mt-3 flex flex-wrap items-center gap-2">
                <PlatformTypeBadge
                  :platform="account.platform"
                  :type="account.type"
                  :plan-type="account.account_level && account.account_level !== 'unknown' ? account.account_level : undefined"
                />
              </div>
            </div>
          </div>
        </section>

        <section class="rounded-2xl border border-[#d9d9e3] bg-white/95 p-5 dark:border-[#3f3f46] dark:bg-[#171717]">
          <div class="mb-4 flex items-center justify-between">
            <h4 class="text-base font-semibold text-[#202123] dark:text-[#ececf1]">
              {{ t('carpool.members') }}
            </h4>
            <div class="flex items-center gap-2">
              <button
                v-if="detailData.summary.is_owner && activeAllocationMembers.length > 0"
                type="button"
                class="btn btn-secondary btn-sm"
                @click="openAllocationDialog()"
              >
                {{ t('carpool.editAllocation') }}
              </button>
              <span class="text-xs text-[#6e6e80] dark:text-[#acacbe]">{{ visibleDetailMembers.length }}</span>
            </div>
          </div>
          <div class="grid grid-cols-1 gap-3 xl:grid-cols-2">
            <div
              v-for="member in visibleDetailMembers"
              :key="member.member.id"
              class="rounded-xl border border-[#d9d9e3] p-4 dark:border-[#3f3f46]"
            >
              <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                <div class="space-y-1">
                  <div class="flex flex-wrap items-center gap-2">
                    <div class="text-sm font-semibold text-[#202123] dark:text-[#ececf1]">
                      {{ member.username || member.masked_email }}
                    </div>
                    <span class="rounded-full bg-[#f3f3f6] px-2 py-0.5 text-[11px] font-medium text-[#565869] dark:bg-[#2f2f2f] dark:text-[#c5c5d2]">
                      {{ memberRoleLabel(member.member.role) }}
                    </span>
                  </div>
                  <div class="text-xs text-[#6e6e80] dark:text-[#acacbe]">
                    {{ member.masked_email }}
                  </div>
                  <div class="text-xs text-[#6e6e80] dark:text-[#acacbe]">
                    {{ t('carpool.currentUserStatus') }}: {{ currentUserStatusLabel(member.member.status) }}
                  </div>
                  <div class="text-xs text-[#6e6e80] dark:text-[#acacbe]">
                    {{ t('carpool.allocationShare') }}: {{ formatPercent(memberAllocationPercent(member)) }}
                  </div>
                </div>

                <button
                  v-if="detailData.summary.is_owner && member.member.role !== 'owner' && member.member.status === 'active'"
                  type="button"
                  class="btn btn-danger btn-sm"
                  :disabled="actingMemberId === member.member.id"
                  @click="handleRemoveMember(member.member.id)"
                >
                  {{ t('carpool.removeMember') }}
                </button>
              </div>

              <div class="mt-3 rounded-xl bg-[#f3f3f6] px-3 py-3 dark:bg-[#171717]">
                <div class="space-y-1.5">
                  <UsageProgressBar
                    label="5h"
                    :utilization="memberUsageWindowUtilization(member, '5h')"
                    :resets-at="memberUsageWindowResetAt(member, '5h')"
                    :show-now-when-idle="true"
                    :show-label="false"
                    color="indigo"
                  />
                  <UsageProgressBar
                    label="7d"
                    :utilization="memberUsageWindowUtilization(member, '7d')"
                    :resets-at="memberUsageWindowResetAt(member, '7d')"
                    :show-now-when-idle="true"
                    :show-label="false"
                    color="emerald"
                  />
                </div>
                <div class="mt-2 grid grid-cols-1 gap-1 sm:grid-cols-2">
                  <div class="flex min-w-0 items-center justify-between gap-2 rounded-lg bg-white/70 px-2 py-1 text-[11px] text-[#6e6e80] dark:bg-[#171717]/70 dark:text-[#c5c5d2]">
                    <span class="min-w-0 truncate font-medium">{{ t('carpool.totalTokens') }}</span>
                    <span class="shrink-0 font-mono">{{ formatTokenCount(member.total_tokens) }}</span>
                  </div>
                  <div class="flex min-w-0 items-center justify-between gap-2 rounded-lg bg-white/70 px-2 py-1 text-[11px] text-[#6e6e80] dark:bg-[#171717]/70 dark:text-[#c5c5d2]">
                    <span class="min-w-0 truncate font-medium">{{ t('carpool.totalCost') }}</span>
                    <span class="shrink-0 font-mono">{{ formatMoney(member.total_cost_usd) }}</span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </section>

        <section
          v-if="detailData.summary.is_owner"
          class="rounded-2xl border border-[#d9d9e3] bg-white/95 p-5 dark:border-[#3f3f46] dark:bg-[#171717]"
        >
          <div class="mb-4 flex items-center justify-between">
            <h4 class="text-base font-semibold text-[#202123] dark:text-[#ececf1]">
              {{ t('carpool.joinRequests') }}
            </h4>
            <span class="text-xs text-[#6e6e80] dark:text-[#acacbe]">{{ detailData.join_requests.length }}</span>
          </div>

          <div v-if="detailData.join_requests.length === 0" class="rounded-xl border border-dashed border-[#c5c5d2] px-4 py-8 text-center text-sm text-[#6e6e80] dark:border-[#3f3f46] dark:text-[#acacbe]">
            {{ t('carpool.noJoinRequests') }}
          </div>
          <div v-else class="space-y-3">
            <div
              v-for="requestProfile in detailData.join_requests"
              :key="requestProfile.request.id"
              class="rounded-xl border border-[#d9d9e3] p-4 dark:border-[#3f3f46]"
            >
              <div class="flex flex-col gap-3 xl:flex-row xl:items-start xl:justify-between">
                <div class="min-w-0 flex-1 space-y-3">
                  <div class="flex flex-wrap items-center gap-2">
                    <div class="text-sm font-semibold text-[#202123] dark:text-[#ececf1]">
                      {{ requestProfile.username || requestProfile.masked_email }}
                    </div>
                    <span :class="requestStatusClass(requestProfile.request.status)" class="rounded-full px-2 py-0.5 text-[11px] font-medium">
                      {{ requestStatusLabel(requestProfile.request.status) }}
                    </span>
                  </div>
                  <div class="text-xs text-[#6e6e80] dark:text-[#acacbe]">
                    {{ requestProfile.masked_email }}
                  </div>
                  <div v-if="requestProfile.request.note" class="rounded-lg bg-[#f3f3f6] px-3 py-2 text-sm text-[#6e6e80] dark:bg-[#171717] dark:text-[#c5c5d2]">
                    {{ requestProfile.request.note }}
                  </div>
	                  <div class="grid grid-cols-2 gap-2 text-xs text-[#6e6e80] dark:text-[#acacbe] md:grid-cols-4">
	                    <div>{{ t('carpool.totalRequests') }}: {{ formatInteger(requestProfile.usage.total_requests) }}</div>
	                    <div>{{ t('carpool.totalTokens') }}: {{ formatTokenCount(requestProfile.usage.total_tokens) }}</div>
	                    <div>{{ t('carpool.last7dTokens') }}: {{ formatTokenCount(requestProfile.usage.last_7d_tokens) }}</div>
	                    <div>{{ t('carpool.last30dTokens') }}: {{ formatTokenCount(requestProfile.usage.last_30d_tokens) }}</div>
	                  </div>
                  <div class="max-w-xl">
                    <label class="input-label">{{ t('carpool.reviewNote') }}</label>
                    <input
                      v-model.trim="reviewNotes[requestProfile.request.id]"
                      class="input"
                      type="text"
                      maxlength="200"
                      :placeholder="t('carpool.reviewNotePlaceholder')"
                    />
                  </div>
                </div>

                <div class="flex flex-wrap gap-2">
                  <button
                    v-if="requestProfile.request.status === 'pending'"
                    type="button"
                    class="btn btn-primary btn-sm"
                    :disabled="actingRequestId === requestProfile.request.id"
                    @click="handleApprove(requestProfile.request.id)"
                  >
                    {{ t('carpool.approve') }}
                  </button>
                  <button
                    v-if="requestProfile.request.status === 'pending'"
                    type="button"
                    class="btn btn-secondary btn-sm"
                    :disabled="actingRequestId === requestProfile.request.id"
                    @click="handleReject(requestProfile.request.id)"
                  >
                    {{ t('carpool.reject') }}
                  </button>
                  <button
                    v-if="requestProfile.request.status === 'approved'"
                    type="button"
                    class="btn btn-primary btn-sm"
                    :disabled="actingRequestId === requestProfile.request.id"
                    @click="handleConfirmPaid(requestProfile.request.id)"
                  >
                    {{ t('carpool.confirmPaid') }}
                  </button>
                </div>
              </div>
            </div>
          </div>
        </section>
      </div>
    </BaseDialog>

    <ConfirmDialog
      :show="showDeleteConfirm"
      :title="t('carpool.deleteDialogTitle')"
      :message="t('carpool.deleteConfirm', { name: deleteTargetSummary?.pool.name || '' })"
      :confirm-text="t('carpool.deletePool')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="confirmDeletePool"
      @cancel="closeDeleteConfirm"
    />

    <ConfirmDialog
      :show="showResetLimitConfirm"
      :title="t('carpool.resetLocalLimitDialogTitle')"
      :message="t('carpool.resetLocalLimitConfirm', { name: resetLimitTarget?.accountName || '' })"
      :confirm-text="t('carpool.resetLocalLimit')"
      :cancel-text="t('common.cancel')"
      @confirm="confirmResetAccountLocalLimit"
      @cancel="closeResetLimitConfirm"
    />

    <BaseDialog
      :show="showAllocationDialog"
      :title="t('carpool.allocationDialogTitle')"
      width="wide"
      :z-index="60"
      @close="closeAllocationDialog"
    >
      <form v-if="detailData" class="space-y-4" @submit.prevent="submitMemberAllocations">
        <div class="rounded-xl border border-[#d9d9e3] bg-[#ffffff] p-4 text-sm text-[#6e6e80] dark:border-[#3f3f46] dark:bg-[#171717] dark:text-[#c5c5d2]">
          {{ t('carpool.allocationHint') }}
        </div>

        <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div class="text-sm text-[#6e6e80] dark:text-[#c5c5d2]">
            {{ t('carpool.allocationTotal') }}:
            <span :class="allocationTotalClass" class="font-mono font-semibold">
              {{ allocationTotalPercent.toFixed(2) }}%
            </span>
          </div>
          <button type="button" class="btn btn-secondary btn-sm" @click="applyEqualAllocation">
            {{ t('carpool.equalAllocation') }}
          </button>
        </div>

        <div class="space-y-3">
          <div
            v-for="member in activeAllocationMembers"
            :key="member.member.id"
            class="grid grid-cols-1 gap-3 rounded-xl border border-[#d9d9e3] p-4 dark:border-[#3f3f46] sm:grid-cols-[minmax(0,1fr)_160px]"
          >
            <div class="min-w-0">
              <div class="flex flex-wrap items-center gap-2">
                <span class="truncate text-sm font-semibold text-[#202123] dark:text-[#ececf1]">
                  {{ member.username || member.masked_email }}
                </span>
                <span class="rounded-full bg-[#f3f3f6] px-2 py-0.5 text-[11px] font-medium text-[#565869] dark:bg-[#2f2f2f] dark:text-[#c5c5d2]">
                  {{ memberRoleLabel(member.member.role) }}
                </span>
              </div>
              <div class="mt-1 truncate text-xs text-[#6e6e80] dark:text-[#acacbe]">
                {{ member.masked_email }}
              </div>
            </div>
            <label class="min-w-0">
              <span class="input-label">{{ t('carpool.allocationPercent') }}</span>
              <div class="flex items-center rounded-lg border border-[#c5c5d2] bg-white px-3 py-2 dark:border-[#3f3f46] dark:bg-[#171717]">
                <input
                  v-model="allocationPercents[member.member.id]"
                  class="min-w-0 flex-1 bg-transparent text-right font-mono text-sm text-[#202123] outline-none dark:text-[#ececf1]"
                  type="number"
                  min="0"
                  max="100"
                  step="0.01"
                  required
                />
                <span class="ml-2 text-xs text-[#6e6e80] dark:text-[#acacbe]">%</span>
              </div>
            </label>
          </div>
        </div>

        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="closeAllocationDialog">
            {{ t('common.cancel') }}
          </button>
          <button type="submit" class="btn btn-primary" :disabled="savingAllocations || !allocationTotalValid">
            {{ savingAllocations ? t('common.saving') : t('common.save') }}
          </button>
        </div>
      </form>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import GroupBadge from '@/components/common/GroupBadge.vue'
import PlatformTypeBadge from '@/components/common/PlatformTypeBadge.vue'
import UsageProgressBar from '@/components/account/UsageProgressBar.vue'
import Icon from '@/components/icons/Icon.vue'
import { UiIconButton, UiMetric, UiMetricStrip, UiPage } from '@/ui'
import { accountsAPI, carpoolsAPI } from '@/api'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { extractApiErrorMessage } from '@/utils/apiError'
import { useClipboard } from '@/composables/useClipboard'
import { formatCompactNumber } from '@/utils/format'
import type {
  Account,
  AccountPlatform,
  CarpoolMineOverview,
  CarpoolMemberProfile,
  CarpoolPoolAccount,
  CarpoolPoolDetail,
  CarpoolPoolSummary,
  CarpoolPoolVisibility,
  CarpoolUsageWindow,
} from '@/types'

type TabKey = 'mine' | 'invite'
type CarpoolStatusFilter = 'all' | 'recruiting' | 'full' | 'closed' | 'pending'

interface CreateFormState {
  name: string
  platform: AccountPlatform
  visibility: CarpoolPoolVisibility
  targetSeats: number
  durationDays: number
  seatPrice: number
  useSystemProxyService: boolean
  useRiskControlService: boolean
  notes: string
}

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()
const { copyToClipboard } = useClipboard()

const activeTab = ref<TabKey>('mine')
const loading = ref(false)
const hasLoadedOnce = ref(false)
const detailLoading = ref(false)
const detailPoolId = ref<number | null>(null)
const mineOverview = ref<CarpoolMineOverview | null>(null)
const detailData = ref<CarpoolPoolDetail | null>(null)
const poolDetails = reactive<Record<number, CarpoolPoolDetail>>({})
const loadingPoolDetails = reactive<Record<number, boolean>>({})

const showCreateDialog = ref(false)
const showBindDialog = ref(false)
const showApplyDialog = ref(false)
const showDetailDialog = ref(false)
const showDeleteConfirm = ref(false)
const showAllocationDialog = ref(false)
const showResetLimitConfirm = ref(false)

const creatingPool = ref(false)
const bindingAccounts = ref(false)
const submittingApply = ref(false)
const loadingOwnerAccounts = ref(false)
const resolvingInvite = ref(false)
const actingRequestId = ref<number | null>(null)
const actingMemberId = ref<number | null>(null)
const deletingPoolId = ref<number | null>(null)
const savingAllocations = ref(false)
const resettingAccountId = ref<number | null>(null)

const bindTargetSummary = ref<CarpoolPoolSummary | null>(null)
const applyTarget = ref<CarpoolPoolSummary | null>(null)
const applyInviteCode = ref('')
const deleteTargetSummary = ref<CarpoolPoolSummary | null>(null)
const resetLimitTarget = ref<{
  poolId: number
  accountId: number
  accountName: string
} | null>(null)
const ownerAccounts = ref<Account[]>([])
const selectedBindAccountIds = ref<number[]>([])
const applyNote = ref('')
const inviteInput = ref('')
const reviewNotes = reactive<Record<number, string>>({})
const allocationPercents = reactive<Record<number, string>>({})
const carpoolSearch = ref('')
const carpoolStatusFilter = ref<CarpoolStatusFilter>('all')
const ownedPage = ref(1)
const joinedPage = ref(1)
const expandedPoolIds = ref<Set<number>>(new Set())

const createForm = reactive<CreateFormState>(defaultCreateForm())
const carpoolPageSize = 6

const platformOptions: Array<{ value: AccountPlatform; label: string }> = [
  { value: 'openai', label: 'OpenAI' },
  { value: 'anthropic', label: 'Claude' },
  { value: 'gemini', label: 'Gemini' },
  { value: 'antigravity', label: 'Antigravity' },
]

const seatOptions = [2, 3, 4, 5, 6]
const statusFilterOptions = computed<Array<{ value: CarpoolStatusFilter; label: string }>>(() => [
  { value: 'all', label: t('carpool.filterAllPools') },
  { value: 'recruiting', label: t('carpool.statusRecruiting') },
  { value: 'full', label: t('carpool.statusFull') },
  { value: 'closed', label: t('carpool.statusClosed') },
  { value: 'pending', label: t('carpool.filterPending') },
])

const ownedPools = computed(() => mineOverview.value?.owned ?? [])
const joinedPools = computed(() => mineOverview.value?.joined ?? [])
const filteredOwnedPools = computed(() => filterCarpoolSummaries(ownedPools.value))
const filteredJoinedPools = computed(() => filterCarpoolSummaries(joinedPools.value))
const ownedPageCount = computed(() => pageCount(filteredOwnedPools.value.length))
const joinedPageCount = computed(() => pageCount(filteredJoinedPools.value.length))
const pagedOwnedPools = computed(() => pagedItems(filteredOwnedPools.value, ownedPage.value))
const pagedJoinedPools = computed(() => pagedItems(filteredJoinedPools.value, joinedPage.value))
const carpoolStats = computed(() => {
  const owned = ownedPools.value
  const joined = joinedPools.value
  return {
    owned: owned.length,
    joined: joined.length,
    total: owned.length + joined.length,
    pending: owned.reduce((sum, item) => sum + Number(item.pending_applications || 0), 0)
      + joined.filter((item) => item.current_user_status === 'pending').length,
  }
})

function configuredFee(value: unknown, fallback: number): number {
  const numeric = Number(value)
  return Number.isFinite(numeric) && numeric >= 0 ? numeric : fallback
}

const carpoolBaseServiceFee = computed(() =>
  configuredFee(appStore.cachedPublicSettings?.carpool_base_service_fee_usd, 75),
)
const carpoolSystemProxyFee = computed(() =>
  configuredFee(appStore.cachedPublicSettings?.carpool_system_proxy_fee_usd, 10),
)
const carpoolRiskControlFee = computed(() =>
  configuredFee(appStore.cachedPublicSettings?.carpool_risk_control_fee_usd, 15),
)

function defaultCreateForm(): CreateFormState {
  return {
    name: '',
    platform: 'openai',
    visibility: 'invite_only',
    targetSeats: 3,
    durationDays: 30,
    seatPrice: 0,
    useSystemProxyService: false,
    useRiskControlService: false,
    notes: '',
  }
}

const createExtraFee = computed(() => {
  return carpoolBaseServiceFee.value
    + (createForm.useSystemProxyService ? carpoolSystemProxyFee.value : 0)
    + (createForm.useRiskControlService ? carpoolRiskControlFee.value : 0)
})

const createExtraFeeDescription = computed(() => {
  const selected: string[] = [t('carpool.extraFeeDefaultService', { amount: formatExtraFeeMoney(carpoolBaseServiceFee.value) })]
  if (createForm.useSystemProxyService) {
    selected.push(t('carpool.extraFeeSystemProxy', { amount: formatExtraFeeMoney(carpoolSystemProxyFee.value) }))
  }
  if (createForm.useRiskControlService) {
    selected.push(t('carpool.extraFeeRiskControl', { amount: formatExtraFeeMoney(carpoolRiskControlFee.value) }))
  }
  return selected.join('；')
})

const visibleDetailMembers = computed(() => {
  const detail = detailData.value
  if (!detail) return []
  return detail.members
})

const activeAllocationMembers = computed(() => {
  const detail = detailData.value
  if (!detail) return []
  return detail.members.filter((member) => member.member.status === 'active')
})

const allocationTotalPercent = computed(() => {
  return activeAllocationMembers.value.reduce((sum, member) => {
    return sum + normalizedAllocationPercent(allocationPercents[member.member.id])
  }, 0)
})

const allocationTotalValid = computed(() => {
  return Math.abs(allocationTotalPercent.value - 100) <= 0.01
})

const allocationTotalClass = computed(() => {
  return allocationTotalValid.value
    ? 'text-[#4f7a4c] dark:text-[#b9d8b6]'
    : 'text-[#a6523f] dark:text-[#e6b2a4]'
})

function filterCarpoolSummaries(items: CarpoolPoolSummary[]): CarpoolPoolSummary[] {
  const keyword = carpoolSearch.value.trim().toLowerCase()
  const status = carpoolStatusFilter.value
  return items.filter((item) => {
    if (keyword) {
      const haystack = [
        item.pool.name,
        item.group_name,
        item.pool.platform,
        item.pool.invite_code,
        item.current_user_status,
      ].filter(Boolean).join(' ').toLowerCase()
      if (!haystack.includes(keyword)) return false
    }
    if (status === 'all') return true
    if (status === 'pending') {
      return Number(item.pending_applications || 0) > 0 || item.current_user_status === 'pending'
    }
    return item.pool.status === status
  })
}

function pageCount(total: number): number {
  return Math.max(1, Math.ceil(total / carpoolPageSize))
}

function pagedItems<T>(items: T[], page: number): T[] {
  const start = (Math.max(1, page) - 1) * carpoolPageSize
  return items.slice(start, start + carpoolPageSize)
}

function setOwnedPage(page: number) {
  ownedPage.value = Math.min(Math.max(1, page), ownedPageCount.value)
}

function setJoinedPage(page: number) {
  joinedPage.value = Math.min(Math.max(1, page), joinedPageCount.value)
}

function paginationInfo(page: number, total: number): string {
  if (total <= 0) {
    return t('carpool.paginationInfo', { start: 0, end: 0, total: 0 })
  }
  const start = (Math.max(1, page) - 1) * carpoolPageSize + 1
  const end = Math.min(total, start + carpoolPageSize - 1)
  return t('carpool.paginationInfo', { start, end, total })
}

function poolExpanded(poolId: number): boolean {
  return expandedPoolIds.value.has(poolId)
}

function resetCreateForm() {
  Object.assign(createForm, defaultCreateForm())
}

async function loadOverview() {
  loading.value = true
  try {
    const mine = await carpoolsAPI.listMine()
    mineOverview.value = mine
    hasLoadedOnce.value = true
    void preloadVisiblePoolDetails()
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('carpool.loadFailed')))
  } finally {
    loading.value = false
  }
}

async function reloadDetail() {
  if (!detailPoolId.value) return
  detailLoading.value = true
  try {
    detailData.value = await loadPoolDetail(detailPoolId.value)
  } finally {
    detailLoading.value = false
  }
}

async function openDetailDialog(poolId: number) {
  if (poolId <= 0) return
  detailPoolId.value = poolId
  detailData.value = poolDetails[poolId] ?? null
  showDetailDialog.value = true
  detailLoading.value = true
  try {
    const detail = await loadPoolDetail(poolId)
    if (detail) {
      detailData.value = detail
    }
  } finally {
    detailLoading.value = false
  }
}

function closeDetailDialog() {
  showDetailDialog.value = false
  detailPoolId.value = null
  detailData.value = null
}

async function preloadVisiblePoolDetails() {
  const ids = new Set<number>()
  pagedOwnedPools.value.forEach((summary) => ids.add(summary.pool.id))
  pagedJoinedPools.value.forEach((summary) => ids.add(summary.pool.id))
  await Promise.all(
    Array.from(ids)
      .filter((poolId) => !poolDetails[poolId] && !loadingPoolDetails[poolId])
      .map((poolId) => loadPoolDetail(poolId, true)),
  )
}

async function loadPoolDetail(poolId: number, silent = false): Promise<CarpoolPoolDetail | null> {
  if (poolId <= 0) return null
  loadingPoolDetails[poolId] = true
  try {
    const detail = await carpoolsAPI.getDetail(poolId)
    poolDetails[poolId] = detail
    if (detailData.value?.pool.id === poolId || detailPoolId.value === poolId) {
      detailData.value = detail
    }
    return detail
  } catch (error: unknown) {
    if (!silent) {
      appStore.showError(extractApiErrorMessage(error, t('carpool.loadFailed')))
    }
    return null
  } finally {
    loadingPoolDetails[poolId] = false
  }
}

function poolDetail(poolId: number): CarpoolPoolDetail | null {
  return poolDetails[poolId] ?? null
}

function poolDetailLoading(poolId: number): boolean {
  return Boolean(loadingPoolDetails[poolId])
}

function poolVisibleMembers(detail: CarpoolPoolDetail | null): CarpoolMemberProfile[] {
  if (!detail) return []
  return detail.members.filter((member) => member.member.status === 'active')
}

function currentMember(detail: CarpoolPoolDetail | null): CarpoolMemberProfile | null {
  if (!detail) return null
  const currentUserId = authStore.user?.id
  if (currentUserId) {
    const matched = detail.members.find((member) => member.member.user_id === currentUserId && member.member.status === 'active')
    if (matched) return matched
  }
  return detail.members.find((member) => member.member.user_id !== detail.pool.owner_user_id && member.member.status === 'active') ?? detail.members.find((member) => member.member.status === 'active') ?? null
}

function memberAllocationPercent(member: CarpoolMemberProfile, targetSeats?: number | null): number {
  const ratio = Number(member.member.quota_share_ratio || 0)
  if (Number.isFinite(ratio) && ratio > 0) {
    return ratio * 100
  }
  const seats = Number(targetSeats || detailData.value?.pool.target_seats || activeAllocationMembers.value.length || 0)
  return seats > 0 ? 100 / seats : 0
}

function normalizedAllocationPercent(value: string | number | null | undefined): number {
  const numeric = Number(value || 0)
  if (!Number.isFinite(numeric) || numeric < 0) return 0
  return Math.min(numeric, 100)
}

function resetAllocationFormFromDetail() {
  Object.keys(allocationPercents).forEach((key) => {
    delete allocationPercents[Number(key)]
  })
  const members = activeAllocationMembers.value
  const rounded = members.map((member) => Number(memberAllocationPercent(member).toFixed(2)))
  const roundedTotal = rounded.reduce((sum, value) => sum + value, 0)
  if (rounded.length > 0 && Math.abs(roundedTotal - 100) <= 0.05) {
    rounded[rounded.length - 1] += 100 - roundedTotal
  }
  members.forEach((member, index) => {
    allocationPercents[member.member.id] = rounded[index].toFixed(2)
  })
}

function openAllocationDialog(detail?: CarpoolPoolDetail | null) {
  if (detail) {
    detailData.value = detail
    detailPoolId.value = detail.pool.id
  }
  resetAllocationFormFromDetail()
  showAllocationDialog.value = true
}

function closeAllocationDialog() {
  showAllocationDialog.value = false
}

function applyEqualAllocation() {
  const members = activeAllocationMembers.value
  if (members.length === 0) return
  const equal = 100 / members.length
  members.forEach((member, index) => {
    const value = index === members.length - 1
      ? 100 - equal * (members.length - 1)
      : equal
    allocationPercents[member.member.id] = value.toFixed(2)
  })
}

async function submitMemberAllocations() {
  const poolId = detailPoolId.value ?? detailData.value?.pool.id
  if (!poolId) return
  if (!allocationTotalValid.value) {
    appStore.showError(t('carpool.allocationTotalInvalid'))
    return
  }
  savingAllocations.value = true
  try {
    const detail = await carpoolsAPI.updateMemberAllocations(poolId, {
      allocations: activeAllocationMembers.value.map((member) => ({
        member_id: member.member.id,
        quota_share_ratio: normalizedAllocationPercent(allocationPercents[member.member.id]) / 100,
      })),
    })
    detailData.value = detail
    poolDetails[poolId] = detail
    appStore.showSuccess(t('carpool.allocationSaved'))
    showAllocationDialog.value = false
    await loadOverview()
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('carpool.failedToSaveAllocation')))
  } finally {
    savingAllocations.value = false
  }
}

function openCreateDialog() {
  resetCreateForm()
  showCreateDialog.value = true
}

function closeCreateDialog() {
  showCreateDialog.value = false
}

function normalizeInviteCodeInput(value: string): string {
  const raw = value.trim()
  if (!raw) return ''
  try {
    const url = new URL(raw, typeof window === 'undefined' ? 'https://example.invalid' : window.location.origin)
    const queryCode = url.searchParams.get('invite') || url.searchParams.get('invite_code')
    if (queryCode) return queryCode.trim().toUpperCase()
    const match = url.pathname.match(/\/invite\/([^/]+)/i)
    if (match?.[1]) return decodeURIComponent(match[1]).trim().toUpperCase()
  } catch {
    // Treat plain text as the code.
  }
  return raw.replace(/^#/, '').trim().toUpperCase()
}

async function resolveInviteAndOpenApply() {
  const code = normalizeInviteCodeInput(inviteInput.value)
  if (!code) return
  resolvingInvite.value = true
  try {
    const detail = await carpoolsAPI.getByInviteCode(code)
    detailData.value = detail
    detailPoolId.value = detail.pool.id
    poolDetails[detail.pool.id] = detail
    openApplyDialog(detail.summary, detail.pool.invite_code)
    await router.replace({ query: { ...route.query, invite: undefined, invite_code: undefined } })
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('carpool.failedToResolveInvite')))
  } finally {
    resolvingInvite.value = false
  }
}

async function submitCreate() {
  creatingPool.value = true
  try {
    const created = await carpoolsAPI.createPool({
      name: createForm.name,
      platform: createForm.platform,
      visibility: 'invite_only',
      target_seats: createForm.targetSeats,
      duration_days: createForm.durationDays,
	      seat_price: createForm.seatPrice,
	      extra_fee: createExtraFee.value,
	      extra_fee_description: createExtraFeeDescription.value,
	      system_proxy_enabled: createForm.useSystemProxyService,
	      risk_control_enabled: createForm.useRiskControlService,
	      notes: createForm.notes,
	    })
    appStore.showSuccess(t('carpool.createSuccess'))
    showCreateDialog.value = false
    poolDetails[created.pool.id] = created
    await loadOverview()
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('carpool.failedToCreate')))
  } finally {
    creatingPool.value = false
  }
}

async function reloadOwnerAccounts() {
  if (!bindTargetSummary.value) return
  loadingOwnerAccounts.value = true
  try {
    const response = await accountsAPI.list(1, 1000, {
      platform: bindTargetSummary.value.pool.platform,
      status: 'active',
    })
    ownerAccounts.value = response.items
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('carpool.loadFailed')))
  } finally {
    loadingOwnerAccounts.value = false
  }
}

async function openBindDialog(summary: CarpoolPoolSummary) {
  bindTargetSummary.value = summary
  const cachedDetail = poolDetails[summary.pool.id] ?? (detailData.value?.pool.id === summary.pool.id ? detailData.value : null)
  selectedBindAccountIds.value = cachedDetail
    ? cachedDetail.accounts.map((item) => item.account_id)
    : []
  showBindDialog.value = true
  await reloadOwnerAccounts()
  if (selectedBindAccountIds.value.length === 0 && !cachedDetail) {
    try {
      const detail = await loadPoolDetail(summary.pool.id, true)
      if (!detail) return
      selectedBindAccountIds.value = detail.accounts.map((item) => item.account_id)
    } catch {
      // ignore secondary preload failure; dialog still works with fresh account list
    }
  }
}

function closeBindDialog() {
  showBindDialog.value = false
  bindTargetSummary.value = null
  selectedBindAccountIds.value = []
  ownerAccounts.value = []
}

function toggleBindAccount(accountId: number) {
  if (selectedBindAccountIds.value.includes(accountId)) {
    selectedBindAccountIds.value = selectedBindAccountIds.value.filter((id) => id !== accountId)
    return
  }
  selectedBindAccountIds.value = [...selectedBindAccountIds.value, accountId]
}

async function submitBindAccounts() {
  if (!bindTargetSummary.value) return
  bindingAccounts.value = true
  try {
    const detail = await carpoolsAPI.bindAccounts(bindTargetSummary.value.pool.id, {
      account_ids: selectedBindAccountIds.value,
    })
    poolDetails[detail.pool.id] = detail
    appStore.showSuccess(t('carpool.bindSuccess'))
    showBindDialog.value = false
    await loadOverview()
    if (detailPoolId.value === detail.pool.id) {
      detailData.value = detail
    }
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('carpool.failedToBind')))
  } finally {
    bindingAccounts.value = false
  }
}

function askResetAccountLocalLimit(poolId: number, account: CarpoolPoolAccount) {
  resetLimitTarget.value = {
    poolId,
    accountId: account.account_id,
    accountName: account.name,
  }
  showResetLimitConfirm.value = true
}

function closeResetLimitConfirm() {
  showResetLimitConfirm.value = false
  resetLimitTarget.value = null
}

async function confirmResetAccountLocalLimit() {
  const target = resetLimitTarget.value
  if (!target) return
  showResetLimitConfirm.value = false
  resettingAccountId.value = target.accountId
  try {
    const detail = await carpoolsAPI.resetAccountLocalLimit(target.poolId, target.accountId)
    detailData.value = detail
    poolDetails[target.poolId] = detail
    appStore.showSuccess(t('carpool.resetLocalLimitSuccess'))
    await loadOverview()
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('carpool.failedToResetLocalLimit')))
  } finally {
    resettingAccountId.value = null
    resetLimitTarget.value = null
  }
}

function openApplyDialog(summary: CarpoolPoolSummary, inviteCode = '') {
  applyTarget.value = summary
  applyInviteCode.value = inviteCode
  applyNote.value = ''
  showApplyDialog.value = true
}

function closeApplyDialog() {
  showApplyDialog.value = false
  applyTarget.value = null
  applyInviteCode.value = ''
  applyNote.value = ''
}

async function submitApply() {
  if (!applyTarget.value) return
  submittingApply.value = true
  try {
    if (applyInviteCode.value) {
      await carpoolsAPI.applyByInviteCode(applyInviteCode.value, { note: applyNote.value })
    } else {
      await carpoolsAPI.applyToPool(applyTarget.value.pool.id, { note: applyNote.value })
    }
    appStore.showSuccess(t('carpool.applySuccess'))
    showApplyDialog.value = false
    applyInviteCode.value = ''
    await loadOverview()
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('carpool.failedToApply')))
  } finally {
    submittingApply.value = false
  }
}

async function handleApprove(requestId: number, poolId?: number) {
  const targetPoolId = poolId ?? detailPoolId.value ?? detailData.value?.pool.id
  if (!targetPoolId) return
  actingRequestId.value = requestId
  try {
    await carpoolsAPI.approveJoinRequest(targetPoolId, requestId, {
      review_note: reviewNotes[requestId] || '',
    })
    appStore.showSuccess(t('carpool.approveSuccess'))
    await Promise.all([loadOverview(), loadPoolDetail(targetPoolId)])
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('carpool.failedToApprove')))
  } finally {
    actingRequestId.value = null
  }
}

async function handleReject(requestId: number, poolId?: number) {
  const targetPoolId = poolId ?? detailPoolId.value ?? detailData.value?.pool.id
  if (!targetPoolId) return
  actingRequestId.value = requestId
  try {
    await carpoolsAPI.rejectJoinRequest(targetPoolId, requestId, {
      review_note: reviewNotes[requestId] || '',
    })
    appStore.showSuccess(t('carpool.rejectSuccess'))
    await Promise.all([loadOverview(), loadPoolDetail(targetPoolId)])
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('carpool.failedToReject')))
  } finally {
    actingRequestId.value = null
  }
}

async function handleConfirmPaid(requestId: number, poolId?: number) {
  const targetPoolId = poolId ?? detailPoolId.value ?? detailData.value?.pool.id
  if (!targetPoolId) return
  actingRequestId.value = requestId
  try {
    const detail = await carpoolsAPI.confirmJoinPaid(targetPoolId, requestId)
    detailData.value = detail
    poolDetails[targetPoolId] = detail
    appStore.showSuccess(t('carpool.confirmPaidSuccess'))
    await loadOverview()
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('carpool.failedToConfirmPaid')))
  } finally {
    actingRequestId.value = null
  }
}

async function handleRemoveMember(memberId: number, poolId?: number) {
  const targetPoolId = poolId ?? detailPoolId.value ?? detailData.value?.pool.id
  if (!targetPoolId) return
  actingMemberId.value = memberId
  try {
    const detail = await carpoolsAPI.removeMember(targetPoolId, memberId)
    detailData.value = detail
    poolDetails[targetPoolId] = detail
    appStore.showSuccess(t('carpool.removeMemberSuccess'))
    await loadOverview()
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('carpool.failedToRemoveMember')))
  } finally {
    actingMemberId.value = null
  }
}

function askDeletePool(summary: CarpoolPoolSummary) {
  deleteTargetSummary.value = summary
  showDeleteConfirm.value = true
}

function closeDeleteConfirm() {
  showDeleteConfirm.value = false
  deleteTargetSummary.value = null
}

async function confirmDeletePool() {
  if (!deleteTargetSummary.value) return
  const poolId = deleteTargetSummary.value.pool.id
  deletingPoolId.value = poolId
  try {
    await carpoolsAPI.deletePool(poolId)
    appStore.showSuccess(t('carpool.deleteSuccess'))
    closeDeleteConfirm()
    if (detailPoolId.value === poolId) {
      detailPoolId.value = null
      detailData.value = null
    }
    delete poolDetails[poolId]
    await loadOverview()
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('carpool.failedToDelete')))
  } finally {
    deletingPoolId.value = null
  }
}

async function copyInviteCode(inviteCode: string) {
  await copyToClipboard(inviteCode, t('carpool.copiedInviteCode'))
}

function buildCarpoolInviteLink(inviteCode: string): string {
  const code = encodeURIComponent(inviteCode)
  if (typeof window === 'undefined') return `/accounts/carpools?invite=${code}`
  return `${window.location.origin}/accounts/carpools?invite=${code}`
}

async function copyInviteLink(inviteCode: string) {
  await copyToClipboard(buildCarpoolInviteLink(inviteCode), t('carpool.copiedInviteLink'))
}

function platformLabel(value: string): string {
  const matched = platformOptions.find((item) => item.value === value)
  return matched?.label || value
}

function serviceStatusLabel(enabled: boolean): string {
  return enabled ? t('carpool.serviceEnabled') : t('carpool.serviceDisabled')
}

function serviceStatusBadgeClass(enabled: boolean): string {
  return [
    'rounded-full px-2 py-0.5 text-[11px] font-medium',
    enabled
      ? 'bg-[#eef7ed] text-[#4f7a4c] dark:bg-[#1b281b] dark:text-[#b9d8b6]'
      : 'bg-[#f3f3f6] text-[#6e6e80] dark:bg-[#2f2f2f] dark:text-[#acacbe]',
  ].join(' ')
}

function accountStatusLabel(value?: string): string {
  switch (value) {
    case 'active':
      return t('admin.accounts.status.active')
    case 'inactive':
      return t('admin.accounts.status.inactive')
    case 'disabled':
      return t('admin.accounts.status.disabled')
    case 'error':
      return t('admin.accounts.status.error')
    default:
      return value || '-'
  }
}

function accountStatusBadgeClass(value?: string): string {
  const base = 'rounded-full px-2 py-0.5 text-[11px] font-medium'
  switch (value) {
    case 'active':
      return `${base} bg-[#eef7ed] text-[#4f7a4c] dark:bg-[#1b281b] dark:text-[#b9d8b6]`
    case 'error':
      return `${base} bg-[#fff1ed] text-[#a6523f] dark:bg-[#2d1d18] dark:text-[#e6b2a4]`
    case 'disabled':
    case 'inactive':
      return `${base} bg-[#f2eee8] text-[#766b60] dark:bg-[#24201d] dark:text-[#b8aca0]`
    default:
      return `${base} bg-[#f3f3f6] text-[#565869] dark:bg-[#2f2f2f] dark:text-[#c5c5d2]`
  }
}

function visibilityLabel(_value: string): string {
  return t('carpool.visibilityInviteOnly')
}

function poolStatusLabel(value: string): string {
  switch (value) {
    case 'full':
      return t('carpool.statusFull')
    case 'closed':
      return t('carpool.statusClosed')
    default:
      return t('carpool.statusRecruiting')
  }
}

function requestStatusLabel(value: string): string {
  switch (value) {
    case 'approved':
      return t('carpool.requestApproved')
    case 'rejected':
      return t('carpool.requestRejected')
    case 'activated':
      return t('carpool.requestActivated')
    default:
      return t('carpool.requestPending')
  }
}

function currentUserStatusLabel(value: string): string {
  switch (value) {
    case 'owner':
      return t('carpool.owner')
    case 'active':
      return t('carpool.member')
    case 'member':
      return t('carpool.member')
    case 'approved':
      return t('carpool.requestApproved')
    case 'rejected':
      return t('carpool.requestRejected')
    case 'activated':
      return t('carpool.requestActivated')
    case 'removed':
      return t('carpool.removed')
    default:
      return t('carpool.requestPending')
  }
}

function memberRoleLabel(value: string): string {
  return value === 'owner' ? t('carpool.owner') : t('carpool.member')
}

function poolStatusClass(value: string): string {
  switch (value) {
    case 'full':
      return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
    case 'closed':
      return 'bg-slate-100 text-slate-700 dark:bg-slate-900/30 dark:text-slate-300'
    default:
      return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
  }
}

function requestStatusClass(value: string): string {
  switch (value) {
    case 'approved':
      return 'bg-sky-100 text-sky-700 dark:bg-sky-900/30 dark:text-sky-300'
    case 'rejected':
      return 'bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-300'
    case 'activated':
      return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
    default:
      return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
  }
}

function memberUsagePercent(used: number | null | undefined, limit: number | null | undefined): number {
  const numericLimit = Number(limit || 0)
  if (!Number.isFinite(numericLimit) || numericLimit <= 0) {
    return 0
  }
  const numericUsed = Math.max(0, Number(used || 0))
  return (numericUsed / numericLimit) * 100
}

function memberUsageWindow(member: CarpoolMemberProfile, window: '5h' | '7d'): CarpoolUsageWindow | null {
  return member.usage_windows?.find((item) => item.window === window) ?? null
}

function poolUsageWindow(window: '5h' | '7d'): CarpoolUsageWindow | null {
  return detailData.value?.pool_usage_windows?.find((item) => item.window === window) ?? null
}

function poolUsageWindowFromDetail(detail: CarpoolPoolDetail | null, window: '5h' | '7d'): CarpoolUsageWindow | null {
  return detail?.pool_usage_windows?.find((item) => item.window === window) ?? null
}

function poolUsageWindowUtilization(window: '5h' | '7d'): number {
  return Number(poolUsageWindow(window)?.utilization || 0)
}

function poolUsageWindowResetAt(window: '5h' | '7d'): string | null {
  return poolUsageWindow(window)?.reset_at ?? null
}

function poolDetailUsageUtilization(detail: CarpoolPoolDetail | null, window: '5h' | '7d'): number {
  return Number(poolUsageWindowFromDetail(detail, window)?.utilization || 0)
}

function poolDetailUsageResetAt(detail: CarpoolPoolDetail | null, window: '5h' | '7d'): string | null {
  return poolUsageWindowFromDetail(detail, window)?.reset_at ?? null
}

function currentMemberUsageUtilization(detail: CarpoolPoolDetail | null, window: '5h' | '7d'): number {
  const member = currentMember(detail)
  return member ? memberUsageWindowUtilization(member, window) : 0
}

function currentMemberUsageResetAt(detail: CarpoolPoolDetail | null, window: '5h' | '7d'): string | null {
  const member = currentMember(detail)
  return member ? memberUsageWindowResetAt(member, window) : null
}

function memberUsageWindowUtilization(member: CarpoolMemberProfile, window: '5h' | '7d'): number {
  const usageWindow = memberUsageWindow(member, window)
  if (usageWindow) {
    return Number(usageWindow.utilization || 0)
  }
  return window === '5h'
    ? memberUsagePercent(member.member.five_hour_used_usd, member.member.five_hour_limit_usd)
    : memberUsagePercent(member.weekly_usage_usd, member.weekly_limit_usd)
}

function memberUsageWindowResetAt(member: CarpoolMemberProfile, window: '5h' | '7d'): string | null {
  const usageWindow = memberUsageWindow(member, window)
  if (usageWindow?.reset_at) {
    return usageWindow.reset_at
  }
  return window === '5h' ? memberFiveHourResetAt(member) : (member.weekly_reset_at ?? null)
}

function memberFiveHourResetAt(member: CarpoolMemberProfile): string | null {
  const windowStart = member.member.five_hour_window_start
  if (!windowStart || Number(member.member.five_hour_used_usd || 0) <= 0) {
    return null
  }
  const start = new Date(windowStart)
  if (Number.isNaN(start.getTime())) {
    return null
  }
  return new Date(start.getTime() + 5 * 60 * 60 * 1000).toISOString()
}

function formatMoney(value: number | null | undefined): string {
  return `$${Number(value || 0).toFixed(2)}`
}

function formatPercent(value: number | null | undefined): string {
  return `${Number(value || 0).toFixed(2)}%`
}

function formatExtraFeeMoney(value: number | null | undefined): string {
  return `$${Number(value || 0).toFixed(2)}`
}

function formatInteger(value: number | null | undefined): string {
  return new Intl.NumberFormat().format(Number(value || 0))
}

function formatTokenCount(value: number | null | undefined): string {
  return formatCompactNumber(Number(value || 0), { allowBillions: true })
}

watch([carpoolSearch, carpoolStatusFilter], () => {
  ownedPage.value = 1
  joinedPage.value = 1
})

watch(ownedPageCount, (count) => {
  if (ownedPage.value > count) {
    ownedPage.value = count
  }
})

watch(joinedPageCount, (count) => {
  if (joinedPage.value > count) {
    joinedPage.value = count
  }
})

watch([pagedOwnedPools, pagedJoinedPools], () => {
  void preloadVisiblePoolDetails()
}, { flush: 'post' })

onMounted(async () => {
  void appStore.fetchPublicSettings()
  await loadOverview()
  const routeInvite = route.query.invite || route.query.invite_code
  const rawInvite = Array.isArray(routeInvite) ? routeInvite[0] : routeInvite
  if (rawInvite) {
    activeTab.value = 'invite'
    inviteInput.value = rawInvite
    await resolveInviteAndOpenApply()
  }
})
</script>

<style scoped>
.carpool-command-bar {
  display: flex;
  min-width: 0;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  border-bottom: 1px solid var(--ui-border);
}

.carpool-tabs,
.carpool-actions {
  display: flex;
  min-width: 0;
  align-items: center;
}

.carpool-tabs {
  gap: 1.25rem;
}

.carpool-actions {
  flex: 0 0 auto;
  gap: 0.5rem;
  padding-bottom: 0.625rem;
}

.carpool-tab {
  margin-bottom: -1px;
  padding: 0.75rem 0 0.875rem;
  border: 0;
  border-bottom: 2px solid transparent;
  background: transparent;
  color: var(--ui-text-tertiary);
  font-size: 0.875rem;
  font-weight: 500;
  white-space: nowrap;
}

.carpool-tab:hover {
  color: var(--ui-text);
}

.carpool-tab--active {
  border-bottom-color: var(--ui-text);
  color: var(--ui-text);
}

.carpool-overview {
  padding-bottom: 1.25rem;
  border-bottom: 1px solid var(--ui-border);
}

.carpool-empty {
  padding: 2rem 0;
  border-top: 1px solid var(--ui-border);
  color: var(--ui-text-tertiary);
  font-size: 0.875rem;
}

.carpool-pool-card {
  min-width: 0;
  padding: 1.25rem;
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-lg);
  background: var(--ui-surface);
}

@media (max-width: 640px) {
  .carpool-command-bar {
    align-items: flex-end;
    gap: 0.5rem;
  }

  .carpool-tabs {
    gap: 1rem;
  }

  .carpool-actions {
    gap: 0.25rem;
  }
}
</style>
