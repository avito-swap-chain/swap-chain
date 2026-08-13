import { Status } from '@/shared/ui'
import type { DeliveryStatus } from '../model/types'

// Оранжевого здесь нет: ход за сотрудником ПВЗ в трёх состояниях из четырёх, и подсветка,
// которая горит почти всегда, перестаёт что-либо значить. Полученная вещь — серая: она
// уже никуда не движется.
const STATUS_VIEW: Record<DeliveryStatus, { tone: 'neutral' | 'muted'; label: string }> = {
  AWAITING_PVZ: { tone: 'neutral', label: 'Ждём в ПВЗ' },
  AT_PVZ: { tone: 'neutral', label: 'Принята в ПВЗ' },
  IN_DELIVERY: { tone: 'neutral', label: 'Едет получателю' },
  RECEIVED: { tone: 'muted', label: 'Получена' },
}

/**
 * Тот же путь глазами того, кто вещь ждёт. Статус один, а «едет получателю» человеку,
 * который и есть получатель, ничего не объясняет.
 */
const INCOMING_LABEL: Record<DeliveryStatus, string> = {
  AWAITING_PVZ: 'Ждёт в ПВЗ',
  AT_PVZ: 'Принята в ПВЗ',
  IN_DELIVERY: 'В пути к вам',
  RECEIVED: 'Получена',
}

interface DeliveryStatusLabelProps {
  status: DeliveryStatus
  /** Чья это вещь: `outgoing` — та, что человек отдаёт, `incoming` — та, что ему везут. */
  direction?: 'outgoing' | 'incoming'
}

/** Где вещь физически: домен знает смысл статуса, `shared/ui/Status` — вид. */
export function DeliveryStatusLabel({ status, direction = 'outgoing' }: DeliveryStatusLabelProps) {
  const view = STATUS_VIEW[status]
  const label = direction === 'incoming' ? INCOMING_LABEL[status] : view.label

  return <Status tone={view.tone}>{label}</Status>
}
