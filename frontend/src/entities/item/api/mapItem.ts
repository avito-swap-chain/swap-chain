import type { ItemCondition as ApiCondition, Item as ApiItem } from '@/shared/api/generated/model'
import type { Item, ItemCondition, ItemStatus } from '../model/types'

/**
 * Статус вещи. У бэкенда он про стадию обработки, у нас — про участие в обмене, и
 * `ANALYZING` не сводится ни к одному из наших: вещь уже с желанием, но в подборе её ещё
 * нет. Раньше он показывался как `idle` — карточка предлагала включить обмен вещи, у которой
 * он включён, и выглядела зависшей. Поэтому у ожидания разбора своё состояние.
 *
 * `WITHDRAWN` близок к `idle` — желания нет, в подборе не участвует, — но состоянием
 * остаётся отдельным: человек только что сам снял вещь с обмена, и подтверждение действия
 * важнее того, что карточка выглядит как никогда не включённая.
 */
const STATUS: Record<ApiItem['status'], ItemStatus> = {
  ANALYZING: 'analyzing',
  ACTION_REQUIRED: 'needs_category',
  MATCHING: 'searching',
  LOCKED: 'reserved',
  // `LOCKED` — вещь заморожена в собравшейся цепочке, `EXCHANGED` — обмен уже состоялся.
  // Раньше второго состояния не было, и завершённая цепочка оставляла вещь «в цепочке»
  // навсегда: карточка врала, что вещь всё ещё едет.
  EXCHANGED: 'exchanged',
  WITHDRAWN: 'withdrawn',
}

/** Состояние вещи. С контракта 0.10.0 его хранит бэкенд и требует при создании. */
const CONDITION: Record<ApiCondition, ItemCondition> = {
  NEW: 'new',
  GOOD: 'good',
  USED: 'used',
}

/**
 * Вещь из контракта в нашу модель. Расхождения, которых не закрыть маппингом:
 *
 * - **категория у вещи теперь двух видов:** выбранная человеком и назначенная анализом.
 *   Показываем выбор человека, а пока его нет — то, что определила модель.
 */
export const mapItem = (item: ApiItem): Item => ({
  id: String(item.id),
  title: item.offerTitle,
  // Имя категории приходит вместе со словарём ключевых слов для матчинга, поэтому
  // подпись на карточке собирается из справочника, а не из ответа про вещь.
  categoryId: item.categoryId ?? item.offerCategoryId ?? undefined,
  category: '',
  condition: CONDITION[item.condition],
  // Пустую строку сводим к `undefined`: у бэкенда описание обязательно, но у засеянных
  // и старых вещей оно пустое, а «описание есть, но пустое» интерфейсу нечего показывать.
  description: item.offerDescription || undefined,
  // `imageUrls` объявлен в контракте обязательным массивом, но бэкенд отдаёт null,
  // когда картинок нет: без защиты обращение по индексу роняет весь список.
  photoUrl: item.imageUrls?.[0],
  // Желание списком вариантов — теперь и в контракте. Идентификатор и категорию варианта
  // бэкенд заводит себе сам, интерфейсу от варианта нужна только формулировка.
  wish: item.wishes.map((variant) => variant.description),
  // Кроме одного случая: раздел варианта не определился, и вещь ждёт ответа владельца.
  // Тогда нужен и идентификатор — по нему решение уезжает обратно.
  pendingWishes: item.wishes
    .filter((variant) => variant.categoryId == null)
    .map(({ id, description }) => ({ id, description })),
  status: STATUS[item.status],
})
