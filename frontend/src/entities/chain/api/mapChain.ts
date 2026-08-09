import type {
  Chain as ApiChain,
  ChainParticipant as ApiParticipant,
} from '@/shared/api/generated/model'
import type { Chain, ChainParticipant, ChainStatus, ParticipantStatus } from '../model/types'

/**
 * Статусы цепочки. Контракт (0.4.0) знает три состояния, наша машина — пять:
 *
 * - `PENDING` → `formed` — предложение собрано, участники отвечают;
 * - `ACCEPTED` → `active` — согласны все, дальше передача вещей;
 * - `REJECTED` → `dissolved` — кто-то отказался.
 *
 * `completed` (все подтвердили получение) и `cancelled` (вещь ушла в другую цепочку)
 * с бэкенда прийти не могут: стадии передачи и параллельного подбора в контракте нет.
 * Пока их нет, эти состояния живут только в моках — см. `shared/config/backend`.
 */
const STATUS: Record<ApiChain['status'], ChainStatus> = {
  PENDING: 'formed',
  ACCEPTED: 'active',
  REJECTED: 'dissolved',
}

const PARTICIPANT_STATUS: Record<ApiParticipant['status'], ParticipantStatus> = {
  WAITING: 'pending',
  APPROVED: 'confirmed',
  DECLINED: 'declined',
}

const mapParticipant = (participant: ApiParticipant, meId: number): ChainParticipant => ({
  userId: String(participant.user.id),
  name: participant.user.username,
  givesItem: {
    id: String(participant.giveItem.id),
    title: participant.giveItem.offerTitle,
    photoUrl: participant.giveItem.imageUrls[0],
  },
  status: PARTICIPANT_STATUS[participant.status],
  // Отметки получения в контракте нет — до её появления стадия передачи не двигается
  // на реальных данных, и признак всегда снят.
  receiptConfirmed: false,
  isMe: participant.user.id === meId ? true : undefined,
})

/**
 * Цепочка из контракта в нашу модель. `isMe` проставляется здесь, а не приходит с сервера:
 * кто «я» — вопрос точки зрения, и это ровно то, что делает мок в `asSeenBy`.
 */
export const mapChain = (chain: ApiChain, meId: number): Chain => ({
  id: String(chain.id),
  status: STATUS[chain.status],
  participants: chain.participants.map((participant) => mapParticipant(participant, meId)),
})
