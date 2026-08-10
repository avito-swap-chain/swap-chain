import { Status } from '@/shared/ui'
import type { ItemStatus } from '../model/types'

// Ход не за пользователем ни в одном из состояний, поэтому оранжевого здесь нет:
// «В цепочке» — обычный тёмный статус, «Поиск обмена» — серый фон жизни вещи.
const STATUS_VIEW: Record<ItemStatus, { tone: 'neutral' | 'muted'; label: string } | null> = {
  reserved: { tone: 'neutral', label: 'В цепочке' },
  searching: { tone: 'muted', label: 'Поиск обмена' },
  // Разбор описания занимает секунды, но без подписи вещь выглядит зависшей: желание
  // указано, а в подборе её ещё нет и делать с ней нечего.
  analyzing: { tone: 'muted', label: 'Проверяем описание' },
  // Снятую вещь подписываем, а не молчим: человек только что сделал действие,
  // и подтверждение результата важнее чистоты карточки.
  withdrawn: { tone: 'muted', label: 'Снято с обмена' },
  idle: null,
}

/** Где вещь в обороте обмена: заморожена в цепочке, ищет её или лежит просто так. */
export function ItemStatusLabel({ status }: { status: ItemStatus }) {
  const view = STATUS_VIEW[status]
  if (!view) return null

  return <Status tone={view.tone}>{view.label}</Status>
}
