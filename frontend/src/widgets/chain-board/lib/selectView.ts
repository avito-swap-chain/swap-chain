import { findMe, type Chain } from '@/entities/chain'

export type ChainView =
  'offer' | 'waiting' | 'handoff' | 'completed' | 'dissolved' | 'declined' | 'cancelled'

/**
 * Машина состояний сделки → под-вид экрана: `chain.status × статус моего участника`.
 * Один роут `/exchange/:id` показывает разное — код повторяет продуктовую логику 1:1.
 */
export function selectView(chain: Chain): ChainView {
  switch (chain.status) {
    case 'dissolved':
      return 'dissolved'
    case 'cancelled':
      return 'cancelled'
    case 'completed':
      return 'completed'
    case 'active':
      return 'handoff'
    case 'formed':
      switch (findMe(chain)?.status) {
        case 'confirmed':
          return 'waiting'
        case 'declined':
          return 'declined'
        default:
          // 'pending' — ход за мной. undefined (меня нет в цепочке) отсекает ChainBoard.
          return 'offer'
      }
  }
}
