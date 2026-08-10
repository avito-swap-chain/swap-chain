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

/** Где вещь физически: домен знает смысл статуса, `shared/ui/Status` — вид. */
export function DeliveryStatusLabel({ status }: { status: DeliveryStatus }) {
  const view = STATUS_VIEW[status]

  return <Status tone={view.tone}>{view.label}</Status>
}
