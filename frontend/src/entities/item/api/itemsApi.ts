import { unwrap } from '@/shared/api/fetcher'
import {
  createItem as createItemRequest,
  listItems,
  updateItem,
  uploadMedia,
} from '@/shared/api/generated/endpoints'
import type { Item as ApiItem, ItemList, MediaUpload } from '@/shared/api/generated/model'
import { isBackendConnected } from '@/shared/config/backend'
import { currentPersonaId, PERSONAS } from '@/shared/model/persona'
import { mapItem } from './mapItem'
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
export type ItemDraft = Omit<Item, 'id' | 'status'> & {
  /**
   * Сам файл фотографии — живёт только до создания вещи. `photoUrl` рядом с ним это
   * `blob:`-ссылка для предпросмотра, и бэкенду её отдавать нельзя: он принимает только
   * свои же адреса из `POST /api/v1/media` и внешние http(s).
   */
  photoFile?: File
}

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
  if (isBackendConnected) {
    const { items } = unwrap<ItemList>(await listItems())
    return items.map(mapItem)
  }

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
  const wishes = usableWishes(draft.wish)
  if (wishes.length === 0) throw new Error('Не указано, что хочется взамен')

  if (isBackendConnected) {
    // Фото сначала уезжает в хранилище бэкенда: в вещь идёт выданный им адрес, а не
    // локальная `blob:`-ссылка — на ней создание вещи отваливалось с 422.
    const uploaded = draft.photoFile
      ? unwrap<MediaUpload>(await uploadMedia({ file: draft.photoFile }))
      : undefined

    // Контракт принимает одну строку желания — склеиваем варианты через «или»: так бэк
    // хотя бы увидит все, пока не примет список (см. shared/config/backend).
    const created = unwrap<ApiItem>(
      await createItemRequest({
        offerTitle: draft.title,
        // Категорию и состояние сюда не подмешиваем: описание читает человек, а бэкенд
        // по нему ищет обмен — служебным словам вроде «good» не место ни там, ни там.
        offerDescription: draft.description?.trim() || draft.title,
        wantDescription: wishes.map((wish) => wish.description).join(' или '),
        imageUrls: uploaded ? [uploaded.url] : [],
      }),
    )
    return mapItem(created)
  }

  await delay(400)

  const wish = wishes

  const { photoFile: _, ...fields } = draft
  const item: Item = { ...fields, wish, id: String(nextId++), status: 'searching' }
  const ownerId = currentPersonaId()
  itemsByOwner = { ...itemsByOwner, [ownerId]: [item, ...(itemsByOwner[ownerId] ?? [])] }
  return item
}

/**
 * Снять вещь с обмена. Не удаление: желание убирается, вещь уходит из подбора и снова
 * становится обычным объявлением. Удалять физически нельзя — вещь лежит в завершённых
 * цепочках как то, что человек отдал или получил, и история обменов рассыпалась бы.
 *
 * Предложения с этой вещью бэкенд отменяет сам, поэтому список обменов после снятия устарел.
 * Вещь в собравшейся цепочке снять нельзя: она уже едет через ПВЗ, бэкенд отвечает 409.
 */
export async function withdrawItem(id: string): Promise<Item> {
  if (isBackendConnected) {
    return mapItem(unwrap<ApiItem>(await updateItem(Number(id), { withdraw: true })))
  }

  await delay(400)

  const ownerId = currentPersonaId()
  const items = itemsByOwner[ownerId] ?? []
  const item = items.find((i) => i.id === id)
  if (!item) throw new Error(`Объявление ${id} не найдено`)

  const updated: Item = { ...item, wish: [], status: 'withdrawn' }
  itemsByOwner = { ...itemsByOwner, [ownerId]: items.map((i) => (i.id === id ? updated : i)) }
  return updated
}

/** Что человек меняет в уже размещённом объявлении. */
export interface ItemEdit {
  /** Желание целиком: форма отдаёт все варианты сразу, правки одного отдельно нет. */
  wish: Wish[]
  /**
   * Пустое описание не отправляем: в контракте у него `minLength: 1`, и стереть его нельзя.
   * Значит поле, оставленное пустым, — это «не трогать», а не «убрать».
   */
  description?: string
}

/**
 * Правка размещённого объявления: описание и желание. Ими же включается обмен у объявления,
 * которое лежало без него, — для бэкенда это один и тот же `PATCH`.
 *
 * Название и фото сюда не входят: в `UpdateItemRequest` (0.6.0) таких полей нет, и менять
 * их пока негде. Каждый вариант желания — отдельное ребро графа, поэтому без желания
 * объявление в подборе не участвует и пустым его не сохраняем.
 */
export async function editItem(id: string, { wish, description }: ItemEdit): Promise<Item> {
  const usable = usableWishes(wish)
  if (usable.length === 0) throw new Error('Не указано, что хочется взамен')

  const text = description?.trim()

  if (isBackendConnected) {
    // Контракт принимает одну строку желания — склеиваем варианты так же, как при создании.
    return mapItem(
      unwrap<ApiItem>(
        await updateItem(Number(id), {
          wantDescription: usable.map((variant) => variant.description).join(' или '),
          ...(text ? { offerDescription: text } : {}),
        }),
      ),
    )
  }

  await delay(400)

  const ownerId = currentPersonaId()
  const items = itemsByOwner[ownerId] ?? []
  const item = items.find((i) => i.id === id)
  if (!item) throw new Error(`Объявление ${id} не найдено`)

  // Вещь, уже попавшую в цепочку, из неё не вынимаем — уточнение желания её не расколдовывает.
  const updated: Item = {
    ...item,
    wish: usable,
    description: text || item.description,
    // Снятая вещь возвращается в подбор так же, как никогда не включённая.
    status: item.status === 'idle' || item.status === 'withdrawn' ? 'searching' : item.status,
  }
  itemsByOwner = { ...itemsByOwner, [ownerId]: items.map((i) => (i.id === id ? updated : i)) }
  return updated
}

/**
 * Включить обмен у уже размещённого объявления — главный вход в сервис: человек не заводит
 * вещь заново, а указывает, что готов её обменять. Описание у такого объявления уже есть,
 * меняется только желание.
 */
export const setItemWish = (id: string, wish: Wish[]): Promise<Item> => editItem(id, { wish })
