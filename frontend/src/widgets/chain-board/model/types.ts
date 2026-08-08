import type { Chain, ChainParticipant, Neighbours } from '@/entities/chain'

/** Данные, общие для всех под-видов доски цепочки. */
export interface ChainViewProps {
  chain: Chain
  /** Участник, за которого играет текущий пользователь. */
  me: ChainParticipant
  /** Кому отдаём вещь и от кого получаем. */
  neighbours: Neighbours
}
