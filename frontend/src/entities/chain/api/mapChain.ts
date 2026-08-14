import type {
  Chain as ApiChain,
  ChainParticipant as ApiParticipant,
} from '@/shared/api/generated/model'
import type { Chain, ChainParticipant, ChainStatus, ParticipantStatus } from '../model/types'

/**
 * Статусы цепочки. Контракт (0.6.0) знает четыре состояния, наша машина — пять:
 *
 * - `PENDING` → `formed` — предложение собрано, участники отвечают;
 * - `ACCEPTED` → `active` — согласны все, дальше передача вещей;
 * - `COMPLETED` → `completed` — получение отметили все, обмен закрыт;
 * - `REJECTED` → `dissolved` — кто-то отказался.
 *
 * `cancelled` (вещь ушла в другую цепочку) с бэкенда прийти по-прежнему не может:
 * параллельного подбора в контракте нет, конкурирующие цепочки он отклоняет как
 * `REJECTED` с причиной в событии. Пока так — состояние живёт только в моках.
 */
const STATUS: Record<ApiChain['status'], ChainStatus> = {
  PENDING: 'formed',
  ACCEPTED: 'active',
  COMPLETED: 'completed',
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
  receiptConfirmed: participant.receiptConfirmed,
  incomingDelivery: participant.incomingDeliveryStatus,
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
  expiresAt: chain.expiresAt,
  // Кто именно отметил получение, теперь приходит полем участника — до этого приходилось
  // выводить отметку из `COMPLETED`, то есть «либо никто, либо все», и прогресс передачи
  // показать было нечем.
  participants: inCycleOrder(chain.participants).map((participant) =>
    mapParticipant(participant, meId),
  ),
})
