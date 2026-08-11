import { currentPersonaId, PERSONAS } from '@/shared/model/persona'
import type { Item, ItemDraft, RecognizedItem, Wish } from '../model/types'

/**
 * Демо-объявления и операции над ними: то, что делал бы бэкенд, если бы он был подключён.
 * Лежат отдельно от `itemsApi`, чтобы в нём остались только реальные вызовы.
 *
 * Объявления разложены по владельцам: у каждой персоны свой кабинет. Зарезервированные вещи —
 * те, что участвуют в цепочках из `chainMocks`, id совпадают. У каждой персоны есть объявление
 * в статусе `idle` — обычное объявление Авито, у которого обмен ещё не включён. Это точка входа
 * в сервис, и каждое такое объявление кому-то из участников нужно: монитор ищет Марк, умные
 * часы — Даша, приставку — Даша. Варианты желания подобраны так, чтобы хотя бы один указывал
 * на вещь, которая у кого-то из персон действительно есть: иначе лишний вариант ничего не даёт
 * и цикл по нему не замкнётся.
 */

const [DASHA, MARK, LENA] = PERSONAS

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

const myItems = () => itemsByOwner[currentPersonaId()] ?? []

/** Найти свою вещь: чужие id в кабинете не ищутся — у каждой персоны свой список. */
function find(id: string): Item {
  const item = myItems().find((candidate) => candidate.id === id)
  if (!item) throw new Error(`Объявление ${id} не найдено`)
  return item
}

function save(updated: Item): Item {
  const ownerId = currentPersonaId()
  itemsByOwner = {
    ...itemsByOwner,
    [ownerId]: (itemsByOwner[ownerId] ?? []).map((item) =>
      item.id === updated.id ? updated : item,
    ),
  }
  return updated
}

export async function listMyItems(): Promise<Item[]> {
  await delay(300)
  return myItems()
}

/**
 * Объявления остальных пользователей — то, из чего вообще может собраться цепочка.
 * Наружу из сущности не отдаём: это внутренность мока, на бэке такой отбор делает поиск.
 */
export function itemsOfOthers(): Item[] {
  const me = currentPersonaId()

  return Object.entries(itemsByOwner)
    .filter(([ownerId]) => ownerId !== me)
    .flatMap(([, items]) => items)
}

/** Желание уже очищено вызывающим: пустые и повторяющиеся варианты отсеивает `itemsApi`. */
export async function create(draft: ItemDraft, wish: Wish[]): Promise<Item> {
  await delay(400)

  const { photoFile: _, ...fields } = draft
  const item: Item = { ...fields, wish, id: String(nextId++), status: 'searching' }
  const ownerId = currentPersonaId()
  itemsByOwner = { ...itemsByOwner, [ownerId]: [item, ...(itemsByOwner[ownerId] ?? [])] }
  return item
}

export async function edit(
  id: string,
  wish: Wish[],
  description?: string,
  title?: string,
): Promise<Item> {
  await delay(400)

  const item = find(id)

  // Вещь, уже попавшую в цепочку, из неё не вынимаем — уточнение желания её не расколдовывает.
  return save({
    ...item,
    wish,
    title: title || item.title,
    description: description || item.description,
    // Снятая вещь возвращается в подбор так же, как никогда не включённая.
    status: item.status === 'idle' || item.status === 'withdrawn' ? 'searching' : item.status,
  })
}

export async function withdraw(id: string): Promise<Item> {
  await delay(400)

  return save({ ...find(id), wish: [], status: 'withdrawn' })
}

/**
 * Распознавание вещи по фото на моках: фрагмент имени файла → правдоподобный результат.
 * Ключи покрывают демо-фотографии из `public/mock/items`; на остальных функция честно
 * отказывается, а не выдумывает вещь.
 *
 * В отличие от бэкенда, здесь есть и название: демо должно показывать сценарий целиком,
 * а модель названий не даёт (см. `recognizeItem`).
 */
const RECOGNIZED: Record<string, RecognizedItem> = {
  bike: { title: 'Горный велосипед', category: 'Спорт и отдых', condition: 'good' },
  dumbbells: { title: 'Гантели 20 кг', category: 'Спорт и отдых', condition: 'used' },
  scooter: { title: 'Электросамокат', category: 'Транспорт', condition: 'good' },
  monitor24: { title: 'Монитор 24"', category: 'Электроника', condition: 'good' },
  monitor: { title: 'Монитор LG 27" IPS', category: 'Электроника', condition: 'good' },
  console: { title: 'Игровая приставка PlayStation 4', category: 'Электроника', condition: 'used' },
  camera: { title: 'Плёночный фотоаппарат', category: 'Электроника', condition: 'used' },
  keyboard: { title: 'Механическая клавиатура', category: 'Электроника', condition: 'new' },
  watch: { title: 'Умные часы Amazfit GTR', category: 'Электроника', condition: 'good' },
  phone: { title: 'Смартфон', category: 'Электроника', condition: 'good' },
  headphones: { title: 'Наушники', category: 'Аудио', condition: 'good' },
  guitar: { title: 'Акустическая гитара', category: 'Хобби и творчество', condition: 'good' },
  coffee: { title: 'Кофеварка', category: 'Дом и дача', condition: 'good' },
  grinder: { title: 'Кофемолка', category: 'Дом и дача', condition: 'good' },
  lamp: { title: 'Настольная лампа', category: 'Дом и дача', condition: 'good' },
  blanket: { title: 'Плед', category: 'Дом и дача', condition: 'new' },
  beanbag: { title: 'Кресло-мешок', category: 'Дом и дача', condition: 'good' },
}

/** Пауза перед ответом: за неё состояние «распознаём» успевает стать видимым. */
const RECOGNIZE_MS = 1700

export async function recognize(file: File, delayMs = RECOGNIZE_MS): Promise<RecognizedItem> {
  await delay(delayMs)

  const name = file.name.toLowerCase()

  // Длинные ключи проверяем первыми: `monitor24.jpg` подходит и под `monitor`.
  const key = Object.keys(RECOGNIZED)
    .sort((a, b) => b.length - a.length)
    .find((candidate) => name.includes(candidate))

  if (!key) throw new Error('Не удалось распознать вещь на фото')

  return RECOGNIZED[key]
}
