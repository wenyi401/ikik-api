import type { StoreOrder } from '@/types/store'

type StoreDrawRewardOrder = Pick<StoreOrder, 'draw_reward_amount' | 'draw_reward_type' | 'product_type'>

export function formatStoreDrawReward(order: StoreDrawRewardOrder): string {
  const amount = Number(order.draw_reward_amount || 0)
  if (order.draw_reward_type === 'points' || order.product_type === 'points_draw') {
    return amount.toFixed(10).replace(/\.?0+$/, '') || '0'
  }
  return `$${amount.toFixed(2)}`
}
