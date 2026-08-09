import type { Item as ApiItem } from '@/shared/api/generated/model'
import type { Item, ItemStatus } from '../model/types'

/**
 * Статус вещи. У бэкенда он про стадию обработки (`ANALYZING` — идёт разбор описания),
 * у нас — про участие в обмене. Совпадение неполное по смыслу: `ANALYZING` показываем
 * как «обмен ещё не включён», потому что до конца разбора вещь в подборе не участвует.
 */
const STATUS: Record<ApiItem['status'], ItemStatus> = {
  ANALYZING: 'idle',
  MATCHING: 'searching',
  LOCKED: 'reserved',
}

/**
 * Вещь из контракта в нашу модель. Расхождения, которых не закрыть маппингом:
 *
 * - **категории и состояния в контракте нет.** Поля остаются пустыми, интерфейс их не рисует —
 *   выдумывать «хорошее» за пользователя нельзя: состояние вещи это то, ради чего в обмене
 *   вообще пишут друг другу.
 * - **желание в контракте одной строкой.** Разворачиваем в массив из одного варианта: наша
 *   модель шире, и когда бэк примет список, маппер станет тривиальным.
 */
export const mapItem = (item: ApiItem): Item => ({
  id: String(item.id),
  title: item.offerTitle,
  category: '',
  condition: undefined,
  photoUrl: item.imageUrls[0],
  wish: item.wantDescription ? [{ category: '', description: item.wantDescription }] : [],
  status: STATUS[item.status],
})
