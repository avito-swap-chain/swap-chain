import { Status, type StatusTone } from '@/shared/ui'
import type { ItemStatus } from '../model/types'

// Оранжевый только там, где ход за пользователем, и такое состояние ровно одно:
// «В цепочке» — обычный тёмный статус, «Поиск обмена» — серый фон жизни вещи.
const STATUS_VIEW: Record<ItemStatus, { tone: StatusTone; label: string } | null> = {
  reserved: { tone: 'neutral', label: 'В цепочке' },
  searching: { tone: 'muted', label: 'Поиск обмена' },
  // Разбор описания занимает секунды, но без подписи вещь выглядит зависшей: желание
  // указано, а в подборе её ещё нет и делать с ней нечего.
  analyzing: { tone: 'muted', label: 'Проверяем описание' },
  // Единственное, чего вещь сама не переживёт: пока категория желания не выбрана, в подбор
  // она не пойдёт. Поэтому не серая подпись, а обращение к человеку — и глаголом, а не
  // описанием состояния: человеку тут есть что сделать.
  needs_category: { tone: 'attention', label: 'Выбрать категорию' },
  // Терминальное состояние: вещь ушла новому владельцу и в кабинете остаётся историей.
  exchanged: { tone: 'muted', label: 'Обмен завершён' },
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
