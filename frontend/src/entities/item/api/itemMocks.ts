import { currentPersonaId, PERSONAS } from '@/shared/model/persona'
import type {
  Item,
  ItemDraft,
  RecognitionStage,
  RecognizeOptions,
  RecognizedItem,
} from '../model/types'

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
      wish: ['Игровая приставка PlayStation', 'Плёночный фотоаппарат'],
    },
    {
      id: '5',
      title: 'Кофеварка',
      photoUrl: '/mock/items/coffee.jpg',
      category: 'Дом и дача',
      condition: 'good',
      status: 'reserved',
      wish: ['Электросамокат'],
    },
    // Единственная вещь без `photoUrl` — намеренно: объявление без фото на Авито обычное дело,
    // и на ней видно, что плейсхолдер `ItemCard` работает.
    {
      id: '2',
      title: 'Гантели 20 кг',
      category: 'Спорт и отдых',
      condition: 'used',
      status: 'searching',
      wish: ['Умные часы', 'Фитнес-браслет', 'Беговая дорожка'],
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
      wish: ['Горный велосипед'],
    },
    {
      id: '22',
      title: 'Механическая клавиатура',
      photoUrl: '/mock/items/keyboard.jpg',
      category: 'Электроника',
      condition: 'new',
      status: 'searching',
      wish: ['Монитор 27"', 'Графический планшет'],
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
      wish: ['Наушники'],
    },
    {
      id: '33',
      title: 'Кофемолка',
      photoUrl: '/mock/items/grinder.jpg',
      category: 'Дом и дача',
      condition: 'good',
      status: 'searching',
      wish: ['Виниловый проигрыватель', 'Кофеварка'],
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
export async function create(draft: ItemDraft, wish: string[]): Promise<Item> {
  await delay(400)

  const { photoFile: _, ...fields } = draft
  const item: Item = { ...fields, wish, id: String(nextId++), status: 'searching' }
  const ownerId = currentPersonaId()
  itemsByOwner = { ...itemsByOwner, [ownerId]: [item, ...(itemsByOwner[ownerId] ?? [])] }
  return item
}

export async function edit(
  id: string,
  wish: string[],
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
 * а модель названий не даёт (см. `recognizeItem`). Описание, наоборот, — главное, что модель
 * отдаёт на самом деле, и без него на демо нечего было бы подставлять в объявление.
 */
const RECOGNIZED: Record<string, RecognizedItem> = {
  bike: {
    title: 'Горный велосипед',
    category: 'Спорт и отдых',
    condition: 'good',
    description:
      'Горный велосипед с алюминиевой рамой, 21 скорость. Катался два сезона по городу, весной обслужен.',
  },
  dumbbells: {
    title: 'Гантели 20 кг',
    category: 'Спорт и отдых',
    condition: 'used',
    description:
      'Разборные гантели на 20 кг: два грифа и набор блинов. Покрытие местами потёрто, замки держат крепко.',
  },
  scooter: {
    title: 'Электросамокат',
    category: 'Транспорт',
    condition: 'good',
    description:
      'Электросамокат, запас хода около 25 км. Складывается, есть фара и дисковый тормоз, аккумулятор держит заряд.',
  },
  monitor24: {
    title: 'Монитор 24"',
    category: 'Электроника',
    condition: 'good',
    description:
      'Монитор 24 дюйма, разрешение Full HD. Подставка регулируется по наклону, кабели питания и HDMI в комплекте.',
  },
  monitor: {
    title: 'Монитор LG 27" IPS',
    category: 'Электроника',
    condition: 'good',
    description:
      'Монитор 27 дюймов, матрица IPS, разрешение 2560×1440. Битых пикселей нет, в комплекте кабель DisplayPort.',
  },
  console: {
    title: 'Игровая приставка PlayStation 4',
    category: 'Электроника',
    condition: 'used',
    description:
      'Игровая приставка с одним джойстиком и кабелями. Стояла в тумбе, работает тихо, диски читает без сбоев.',
  },
  camera: {
    title: 'Плёночный фотоаппарат',
    category: 'Электроника',
    condition: 'used',
    description:
      'Плёночный фотоаппарат с механическим затвором. Объектив чистый, без грибка; чехол и ремень в комплекте.',
  },
  keyboard: {
    title: 'Механическая клавиатура',
    category: 'Электроника',
    condition: 'new',
    description:
      'Механическая клавиатура полного размера, белая подсветка. Кабель отсоединяется, в комплекте съёмник клавиш.',
  },
  watch: {
    title: 'Умные часы Amazfit GTR',
    category: 'Электроника',
    condition: 'good',
    description:
      'Умные часы с круглым экраном и силиконовым ремешком. Держат заряд около недели, зарядка в комплекте.',
  },
  phone: {
    title: 'Смартфон',
    category: 'Электроника',
    condition: 'good',
    description:
      'Смартфон в тёмном корпусе, экран без трещин и царапин. Аккумулятор держит день, чехол в комплекте.',
  },
  headphones: {
    title: 'Наушники',
    category: 'Аудио',
    condition: 'good',
    description:
      'Беспроводные наушники с чехлом-зарядкой. Звук чистый, амбушюры целые, заряда хватает на несколько часов.',
  },
  guitar: {
    title: 'Акустическая гитара',
    category: 'Хобби и творчество',
    condition: 'good',
    description:
      'Акустическая гитара, корпус дредноут. Гриф ровный, строй держит; чехол и запасные струны в комплекте.',
  },
  coffee: {
    title: 'Кофеварка',
    category: 'Дом и дача',
    condition: 'good',
    description:
      'Рожковая кофеварка с капучинатором, давление 15 бар. Рожок и мерная ложка на месте, накипь чистилась.',
  },
  grinder: {
    title: 'Кофемолка',
    category: 'Дом и дача',
    condition: 'good',
    description:
      'Кофемолка с жерновами и регулировкой помола. Работает тихо, контейнер без трещин, шнур целый.',
  },
  lamp: {
    title: 'Настольная лампа',
    category: 'Дом и дача',
    condition: 'good',
    description:
      'Настольная лампа на гибкой ножке, тёплый свет. Есть регулировка яркости, лампочка в комплекте.',
  },
  blanket: {
    title: 'Плед',
    category: 'Дом и дача',
    condition: 'new',
    description:
      'Плед из мягкой шерсти, полутораспальный размер. Не колется и не скатался — пролежал в шкафу.',
  },
  beanbag: {
    title: 'Кресло-мешок',
    category: 'Дом и дача',
    condition: 'good',
    description:
      'Кресло-мешок с плотным чехлом, наполнитель — пенополистирол. Чехол снимается и стирается.',
  },
}

/**
 * Пауза перед ответом: за неё экран разбора успевает показать все три шага, по секунде
 * на каждый. С настоящей моделью это занимает от нескольких секунд до полуминуты, так что
 * пауза здесь не замедляет демо, а показывает его ближе к тому, как оно работает на стенде.
 */
const RECOGNIZE_MS = 2600

/**
 * Доли паузы по шагам. На моках грузить и подключаться некуда, но шаги проходим те же:
 * демо без бэкенда показывает тот же сценарий, что и стенд, — иначе экрана разбора там
 * не увидеть вовсе.
 */
const STAGE_SHARE: [RecognitionStage, number][] = [
  ['upload', 0.25],
  ['connect', 0.3],
  ['analyze', 0.45],
]

export async function recognize(
  file: File,
  { delayMs = RECOGNIZE_MS, onStage, signal }: RecognizeOptions & { delayMs?: number } = {},
): Promise<RecognizedItem> {
  for (const [stage, share] of STAGE_SHARE) {
    onStage?.(stage)
    await delay(delayMs * share)
    if (signal?.aborted) throw new DOMException('Распознавание отменено', 'AbortError')
  }

  const name = file.name.toLowerCase()

  // Длинные ключи проверяем первыми: `monitor24.jpg` подходит и под `monitor`.
  const key = Object.keys(RECOGNIZED)
    .sort((a, b) => b.length - a.length)
    .find((candidate) => name.includes(candidate))

  if (!key) throw new Error('Не удалось распознать вещь на фото')

  return RECOGNIZED[key]
}
