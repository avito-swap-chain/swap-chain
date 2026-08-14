import { unwrap } from '@/shared/api/fetcher'
import {
  createItem as createItemRequest,
  listItems,
  listUserItems,
  resolveItemCategories,
  updateItem,
  uploadMedia,
} from '@/shared/api/generated/endpoints'
import type {
  Item as ApiItem,
  ItemCondition as ApiItemCondition,
  ItemList,
  MediaUpload,
} from '@/shared/api/generated/model'
import { isBackendConnected } from '@/shared/config/backend'
import * as mock from './itemMocks'
import { mapItem } from './mapItem'
import type { Item, ItemCondition, ItemDraft } from '../model/types'

/** Ключи кэша TanStack Query для вещей. */
export const itemKeys = {
  my: () => ['my-items'] as const,
  ofUser: (userId: string) => ['user-items', userId] as const,
  /**
   * Подсказки желания зависят и от вещи, и от уже выбранных вариантов: выбранный вариант
   * из подсказок уходит, а на его место встаёт следующий кандидат.
   */
  suggestions: (title: string, chosen: string[]) => ['wish-suggestions', title, chosen] as const,
}

/** Что человек меняет в уже размещённом объявлении. */
export interface ItemEdit {
  /** Желание целиком: форма отдаёт все варианты сразу, правки одного отдельно нет. */
  wish: string[]
  /**
   * Пустое описание не отправляем: в контракте у него `minLength: 1`, и стереть его нельзя.
   * Значит поле, оставленное пустым, — это «не трогать», а не «убрать».
   */
  description?: string
  /** Название. Появилось в контракте позже остального — до этого правка была наполовину. */
  title?: string
  /** Категория справочника. Её смена перезапускает разбор вещи на бэкенде. */
  categoryId?: number
  /** Состояние вещи. Хранится на бэкенде с 0.10.0 — до этого правилось только на моках. */
  condition?: ItemCondition
}

/** Наши состояния в контрактные: у нас строчные, у бэкенда — заглавные. */
const API_CONDITION: Record<ItemCondition, ApiItemCondition> = {
  new: 'NEW',
  good: 'GOOD',
  used: 'USED',
}

/**
 * Вариант без описания — ребро в никуда, в граф он не идёт. Отбрасываем такие молча:
 * добавить строку и передумать — нормальный ход, ошибкой это делать незачем.
 * Повтор того же описания — то же самое ребро, второй раз оно шансов не добавляет.
 */
function usableWishes(wish: string[]): string[] {
  const seen = new Set<string>()

  return wish
    .map((variant) => variant.trim())
    .filter((variant) => {
      const key = variant.toLowerCase()
      if (variant === '' || seen.has(key)) return false
      seen.add(key)
      return true
    })
}

export async function getMyItems(): Promise<Item[]> {
  if (!isBackendConnected) return mock.listMyItems()

  const { items } = unwrap<ItemList>(await listItems())
  return items.map(mapItem)
}

/**
 * Вещи чужого человека — то, что он выставил на обмен. В обмене это довод не слабее рейтинга:
 * прежде чем отдать своё, смотрят, с чем человек вообще пришёл. Появилось в контракте 0.10.0.
 *
 * Одной страницей: в профиле показываем первый десяток, листать чужой каталог здесь незачем.
 */
export async function getUserItems(userId: string): Promise<Item[]> {
  if (!isBackendConnected) return mock.listUserItems(userId)

  const { items } = unwrap<ItemList>(await listUserItems(Number(userId), { limit: 10 }))
  return items.map(mapItem)
}

/** Новая вещь сразу уходит в подбор — сервис ищет для неё цепочку. */
export async function createItem(draft: ItemDraft): Promise<Item> {
  const wish = usableWishes(draft.wish)
  if (wish.length === 0) throw new Error('Не указано, что хочется взамен')
  // Категория обязательна с контракта 0.9.0: по ней идёт подбор, и угадать её за человека
  // бэкенд больше не берётся. Форма без категории дальше не пускает — это страховка.
  if (!draft.categoryId) throw new Error('Не выбрана категория')
  // Состояние обязательно с контракта 0.10.0: без него бэкенд отвечает 400. Спрашиваем
  // его у человека — модель по фото только предлагает, подтверждает всегда он.
  if (!draft.condition) throw new Error('Не выбрано состояние вещи')

  if (!isBackendConnected) return mock.create(draft, wish)

  // Фото сначала уезжает в хранилище бэкенда: в вещь идёт выданный им адрес, а не
  // локальная `blob:`-ссылка — на ней создание вещи отваливалось с 422. Если фото уже
  // загружали ради распознавания, берём тот же адрес и не грузим второй раз.
  const imageUrl =
    draft.uploadedUrl ??
    (draft.photoFile
      ? unwrap<MediaUpload>(await uploadMedia({ file: draft.photoFile })).url
      : undefined)

  const created = unwrap<ApiItem>(
    await createItemRequest({
      offerTitle: draft.title,
      // Состояние в описание не подмешиваем: его читает человек, а бэкенд по нему ищет
      // обмен — служебным словам вроде «good» не место ни там, ни там. Категория едет
      // отдельным полем, для подбора она и нужна.
      offerDescription: draft.description?.trim() || draft.title,
      wishes: wish,
      imageUrls: imageUrl ? [imageUrl] : [],
      categoryId: draft.categoryId,
      condition: API_CONDITION[draft.condition],
    }),
  )
  return mapItem(created)
}

/**
 * Правка размещённого объявления: название, описание и желание. Ими же включается обмен
 * у объявления, которое лежало без него, — для бэкенда это один и тот же `PATCH`.
 *
 * Фото сюда пока не входит: `imageUrls` в `UpdateItemRequest` не принимается. Каждый вариант
 * желания — отдельное ребро графа, поэтому без желания объявление в подборе не участвует
 * и пустым его не сохраняем.
 */
export async function editItem(
  id: string,
  { wish, description, title, categoryId, condition }: ItemEdit,
): Promise<Item> {
  const usable = usableWishes(wish)
  if (usable.length === 0) throw new Error('Не указано, что хочется взамен')

  // Пустые поля не отправляем: у обоих в контракте `minLength: 1`, и стереть их нельзя —
  // значит оставленное пустым означает «не трогать».
  const text = description?.trim()
  const name = title?.trim()

  if (!isBackendConnected) return mock.edit(id, usable, text, name)

  return mapItem(
    unwrap<ApiItem>(
      await updateItem(Number(id), {
        wishes: usable,
        ...(text ? { offerDescription: text } : {}),
        ...(name ? { offerTitle: name } : {}),
        ...(categoryId ? { categoryId } : {}),
        ...(condition ? { condition: API_CONDITION[condition] } : {}),
      }),
    ),
  )
}

/**
 * Включить обмен у уже размещённого объявления — главный вход в сервис: человек не заводит
 * вещь заново, а указывает, что готов её обменять. Описание у такого объявления уже есть,
 * меняется только желание.
 */
export const setItemWish = (id: string, wish: string[]): Promise<Item> => editItem(id, { wish })

/**
 * Досогласовать разделы желаний, которые не определила модель. Вещь стоит в `needs_category`
 * и в подбор не идёт, пока решение не принято, — поэтому бэкенд применяет его целиком,
 * за один запрос по всем нерешённым вариантам.
 *
 * Ветки моков здесь нет намеренно: `needs_category` приходит только из ответа бэкенда,
 * на моках такой вещи не бывает и спрашивать не о чем.
 */
export async function resolveWishCategories(
  id: string,
  decisions: { wishId: number; categoryId: number }[],
): Promise<Item> {
  return mapItem(unwrap<ApiItem>(await resolveItemCategories(Number(id), { wishes: decisions })))
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
  if (!isBackendConnected) return mock.withdraw(id)

  return mapItem(unwrap<ApiItem>(await updateItem(Number(id), { withdraw: true })))
}
