import { needsMyAction, type Chain } from '@/entities/chain'

const STATUS_WEIGHT: Record<Chain['status'], number> = {
  formed: 1,
  active: 2,
  dissolved: 3,
  cancelled: 4,
  completed: 5,
}

const rank = (chain: Chain) => (needsMyAction(chain) ? 0 : STATUS_WEIGHT[chain.status])

/** Сначала то, где ход за пользователем, потом идущие обмены, потом история. */
export const sortByUrgency = (chains: Chain[]): Chain[] =>
  [...chains].sort((a, b) => rank(a) - rank(b))
