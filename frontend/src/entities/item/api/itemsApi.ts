import { currentPersonaId, PERSONAS } from '@/shared/model/persona'
import type { Item, Wish } from '../model/types'

/** Ключи кэша TanStack Query для вещей. */
export const itemKeys = {
  my: () => ['my-items'] as const,
  /**
   * Подсказки желания зависят и от вещи, и от уже выбранных вариантов: выбранный вариант
   * из подсказок уходит, а на его место встаёт следующий кандидат.
   */
  suggestions: (title: string, chosen: string[]) => ['wish-suggestions', title, chosen] as const,
}

/** Что пользователь заполняет в форме: вещь без служебных полей. */
export type ItemDraft = Omit<Item, 'id' | 'status'>

const [DASHA, MARK, LENA] = PERSONAS

// Мок вместо бэкенда — заменяется на реальный запрос к Go-API (TanStack Query).
// Объявления разложены по владельцам: у каждой персоны свой кабинет. Зарезервированные
// вещи — те, что участвуют в цепочках из `chainApi`, id совпадают.
// У каждой персоны есть объявление в статусе `idle` — обычное объявление Авито, у которого
// обмен ещё не включён. Это точка входа в сервис, и каждое такое объявление кому-то из
// участников нужно: монитор ищет Марк, умные часы — Даша, приставку — Даша.
// Варианты желания подобраны так, чтобы хотя бы один указывал на вещь, которая у кого-то из
// персон действительно есть: иначе лишний вариант ничего не даёт и цикл по нему не замкнётся.
let itemsByOwner: Record<string, Item[]> = {
  [DASHA.id]: [
    {
      id: '3',
      title: 'Монитор LG 27" IPS',
      photoUrl: '/mock/items/monitor.jpg',
      category: 'Электроника',
      condition: 'good',
      status: 'idle',
      wish: [],
    },
    {
      id: '1',
      title: 'Горный велосипед',
      photoUrl: '/mock/items/bike.jpg',
      category: 'Спорт и отдых',
      condition: 'good',
      status: 'reserved',
      wish: [
        { category: 'Электроника', description: 'Игровая приставка PlayStation' },
        { category: 'Электроника', description: 'Плёночный фотоаппарат' },
      ],
    },
    {
      id: '5',
      title: 'Кофеварка',
      photoUrl: '/mock/items/coffee.jpg',
      category: 'Дом и дача',
      condition: 'good',
      status: 'reserved',
      wish: [{ category: 'Транспорт', description: 'Электросамокат' }],
    },
    // Единственная вещь без `photoUrl` — намеренно: объявление без фото на Авито обычное дело,
    // и на ней видно, что плейсхолдер `ItemCard` работает.
    {
      id: '2',
      title: 'Гантели 20 кг',
      category: 'Спорт и отдых',
      condition: 'used',
      status: 'searching',
      wish: [
        { category: 'Электроника', description: 'Умные часы' },
        { category: 'Электроника', description: 'Фитнес-браслет' },
        { category: 'Спорт и отдых', description: 'Беговая дорожка' },
      ],
    },
  ],
  [MARK.id]: [
    {
      id: '23',
      title: 'Умные часы Amazfit GTR',
      photoUrl: '/mock/items/watch.jpg',
      category: 'Электроника',
      condition: 'good',
      status: 'idle',
      wish: [],
    },
    {
      id: '21',
      title: 'Наушники',
      photoUrl: '/mock/items/headphones.jpg',
      category: 'Аудио',
      condition: 'good',
      status: 'reserved',
      wish: [{ category: 'Спорт и отдых', description: 'Горный велосипед' }],
    },
    {
      id: '22',
      title: 'Механическая клавиатура',
      photoUrl: '/mock/items/keyboard.jpg',
      category: 'Электроника',
      condition: 'new',
      status: 'searching',
      wish: [
        { category: 'Электроника', description: 'Монитор 27"' },
        { category: 'Электроника', description: 'Графический планшет' },
      ],
    },
  ],
  [LENA.id]: [
    {
      id: '34',
      title: 'PlayStation 4 Slim, 1 ТБ',
      photoUrl: '/mock/items/console.jpg',
      category: 'Электроника',
      condition: 'used',
      status: 'idle',
      wish: [],
    },
    {
      id: '31',
      title: 'Плёночный фотоаппарат',
      photoUrl: '/mock/items/camera.jpg',
      category: 'Электроника',
      condition: 'used',
      status: 'reserved',
      wish: [{ category: 'Аудио', description: 'Наушники' }],
    },
    {
      id: '33',
      title: 'Кофемолка',
      photoUrl: '/mock/items/grinder.jpg',
      category: 'Дом и дача',
      condition: 'good',
      status: 'searching',
      wish: [
        { category: 'Хобби и творчество', description: 'Виниловый проигрыватель' },
        { category: 'Дом и дача', description: 'Кофеварка' },
      ],
    },
  ],
}

let nextId = 100

const delay = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms))

/**
 * Вариант без описания — ребро в никуда, в граф он не идёт. Отбрасываем такие молча:
 * добавить строку и передумать — нормальный ход, ошибкой это делать незачем.
 * Повтор того же описания — то же самое ребро, второй раз оно шансов не добавляет.
 */
function usableWishes(wish: Wish[]): Wish[] {
  const seen = new Set<string>()

  return wish
    .map((variant) => ({ ...variant, description: variant.description.trim() }))
    .filter((variant) => {
      const key = variant.description.toLowerCase()
      if (variant.description === '' || seen.has(key)) return false
      seen.add(key)
      return true
    })
}

export async function getMyItems(): Promise<Item[]> {
  await delay(300)
  return itemsByOwner[currentPersonaId()] ?? []
}

/**
 * Объявления остальных пользователей — то, из чего вообще может собраться цепочка.
 * Наружу из entity не отдаём: это внутренность мока, на бэке такой отбор делает поиск.
 */
export function itemsOfOthers(): Item[] {
  const me = currentPersonaId()

  return Object.entries(itemsByOwner)
    .filter(([ownerId]) => ownerId !== me)
    .flatMap(([, items]) => items)
}

/** Новая вещь сразу уходит в подбор — сервис ищет для неё цепочку. */
export async function createItem(draft: ItemDraft): Promise<Item> {
  await delay(400)

  const wish = usableWishes(draft.wish)
  if (wish.length === 0) throw new Error('Не указано, что хочется взамен')

  const item: Item = { ...draft, wish, id: String(nextId++), status: 'searching' }
  const ownerId = currentPersonaId()
  itemsByOwner = { ...itemsByOwner, [ownerId]: [item, ...(itemsByOwner[ownerId] ?? [])] }
  return item
}

/**
 * Включить обмен у уже размещённого объявления — главный вход в сервис: человек не заводит
 * вещь заново, а указывает, что готов её обменять. Без желания включать нечего: каждый
 * вариант желания — отдельное ребро графа, по которому ищется цепочка.
 */
export async function setItemWish(id: string, wish: Wish[]): Promise<Item> {
  await delay(400)

  const usable = usableWishes(wish)
  if (usable.length === 0) throw new Error('Не указано, что хочется взамен')

  const ownerId = currentPersonaId()
  const items = itemsByOwner[ownerId] ?? []
  const item = items.find((i) => i.id === id)
  if (!item) throw new Error(`Объявление ${id} не найдено`)

  // Вещь, уже попавшую в цепочку, из неё не вынимаем — уточнение желания её не расколдовывает.
  const updated: Item = {
    ...item,
    wish: usable,
    status: item.status === 'idle' ? 'searching' : item.status,
  }
  itemsByOwner = { ...itemsByOwner, [ownerId]: items.map((i) => (i.id === id ? updated : i)) }
  return updated
}
