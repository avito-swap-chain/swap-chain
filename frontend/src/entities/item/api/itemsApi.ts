import { unwrap } from '@/shared/api/fetcher'
import {
  createItem as createItemRequest,
  listItems,
  updateItem,
  uploadMedia,
} from '@/shared/api/generated/endpoints'
import type { Item as ApiItem, ItemList, MediaUpload } from '@/shared/api/generated/model'
import { isBackendConnected } from '@/shared/config/backend'
import * as mock from './itemMocks'
import { mapItem } from './mapItem'
import type { Item, ItemDraft, Wish } from '../model/types'

/** Ключи кэша TanStack Query для вещей. */
export const itemKeys = {
  my: () => ['my-items'] as const,
  /**
   * Подсказки желания зависят и от вещи, и от уже выбранных вариантов: выбранный вариант
   * из подсказок уходит, а на его место встаёт следующий кандидат.
   */
  suggestions: (title: string, chosen: string[]) => ['wish-suggestions', title, chosen] as const,
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

/** Контракт принимает одну строку желания — склеиваем варианты через «или». */
const asWantDescription = (wish: Wish[]) => wish.map((variant) => variant.description).join(' или ')

export async function getMyItems(): Promise<Item[]> {
  if (!isBackendConnected) return mock.listMyItems()

  const { items } = unwrap<ItemList>(await listItems())
  return items.map(mapItem)
}

/** Новая вещь сразу уходит в подбор — сервис ищет для неё цепочку. */
export async function createItem(draft: ItemDraft): Promise<Item> {
  const wish = usableWishes(draft.wish)
  if (wish.length === 0) throw new Error('Не указано, что хочется взамен')

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
      // Категорию и состояние сюда не подмешиваем: описание читает человек, а бэкенд
      // по нему ищет обмен — служебным словам вроде «good» не место ни там, ни там.
      offerDescription: draft.description?.trim() || draft.title,
      wantDescription: asWantDescription(wish),
      imageUrls: imageUrl ? [imageUrl] : [],
    }),
  )
  return mapItem(created)
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

  if (!isBackendConnected) return mock.edit(id, usable, text)

  return mapItem(
    unwrap<ApiItem>(
      await updateItem(Number(id), {
        wantDescription: asWantDescription(usable),
        ...(text ? { offerDescription: text } : {}),
      }),
    ),
  )
}

/**
 * Включить обмен у уже размещённого объявления — главный вход в сервис: человек не заводит
 * вещь заново, а указывает, что готов её обменять. Описание у такого объявления уже есть,
 * меняется только желание.
 */
export const setItemWish = (id: string, wish: Wish[]): Promise<Item> => editItem(id, { wish })

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
