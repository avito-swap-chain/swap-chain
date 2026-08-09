import type {
  Chain as ApiChain,
  ChainParticipant as ApiParticipant,
} from '@/shared/api/generated/model'
import type { Chain, ChainParticipant, ChainStatus, ParticipantStatus } from '../model/types'

/**
 * Статусы цепочки. Контракт (0.5.0) знает три состояния, наша машина — пять:
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
    photoUrl: participant.giveItem.imageUrls?.[0],
  },
  status: PARTICIPANT_STATUS[participant.status],
  // Отметки получения в контракте нет — до её появления стадия передачи не двигается
  // на реальных данных, и признак всегда снят.
  receiptConfirmed: false,
  isMe: participant.user.id === meId ? true : undefined,
})

/**
 * Участники в порядке обхода круга: `participants[i]` отдаёт вещь `participants[i + 1]`.
 * На этом порядке держится весь экран цепочки (`findNeighbours`), но бэкенд его не обещает:
 * он отдаёт участников в порядке вставки (`ORDER BY ci.id`), то есть в порядке рёбер запроса
 * на создание. Порядок восстанавливаем по `receiveItem` — у следующего по кругу это ровно
 * та вещь, которую отдаёт текущий.
 */
function inCycleOrder(participants: ApiParticipant[]): ApiParticipant[] {
  const receiverOf = new Map(participants.map((p) => [p.receiveItem.id, p]))
  const ordered: ApiParticipant[] = []
  let current: ApiParticipant | undefined = participants[0]

  while (current && ordered.length < participants.length) {
    ordered.push(current)
    current = receiverOf.get(current.giveItem.id)
    if (current === participants[0]) break
  }

  // Круг не собрался — данные противоречивы. Оставляем как пришло: порядок будет неверным,
  // но потерять участника хуже, чем показать его не на своём месте.
  return ordered.length === participants.length ? ordered : participants
}

/**
 * Цепочка из контракта в нашу модель. `isMe` проставляется здесь, а не приходит с сервера:
 * кто «я» — вопрос точки зрения, и это ровно то, что делает мок в `asSeenBy`.
 */
export const mapChain = (chain: ApiChain, meId: number): Chain => ({
  id: String(chain.id),
  status: STATUS[chain.status],
  participants: inCycleOrder(chain.participants).map((participant) =>
    mapParticipant(participant, meId),
  ),
})
