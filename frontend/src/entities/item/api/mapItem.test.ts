import { describe, expect, it } from 'vitest'
import type { Item as ApiItem } from '@/shared/api/generated/model'
import { mapItem } from './mapItem'

const apiItem = (over: Partial<ApiItem> = {}): ApiItem => ({
  id: 7,
  userId: 1,
  offerTitle: 'Горный велосипед',
  offerDescription: 'Спорт и отдых, хорошее',
  wishes: [{ id: 1, categoryId: 3, description: 'Игровая приставка' }],
  imageUrls: ['/mock/items/bike.jpg'],
  status: 'MATCHING',
  createdAt: '2026-08-09T10:00:00Z',
  updatedAt: '2026-08-09T10:00:00Z',
  ...over,
})

describe('mapItem — вещь из контракта в нашу модель', () => {
  it('желание из контракта становится вариантом нашей модели', () => {
    expect(mapItem(apiItem()).wish).toEqual(['Игровая приставка'])
  })

  it('пустое желание — это отсутствие вариантов, а не вариант с пустым текстом', () => {
    expect(mapItem(apiItem({ wishes: [] })).wish).toEqual([])
  })

  // Регрессия: пока контракт хранил желание одной строкой, варианты склеивались через
  // « или » и правка объявления открывалась с одним слипшимся полем. Теперь их отдаёт
  // сам бэкенд, и каждый вариант должен остаться отдельной строкой формы.
  it('несколько вариантов желания остаются отдельными', () => {
    const wish = mapItem(
      apiItem({
        wishes: [
          { id: 1, categoryId: 3, description: 'Приставка' },
          { id: 2, categoryId: 4, description: 'Велосипед' },
          { id: 3, categoryId: 5, description: 'Гитара' },
        ],
      }),
    ).wish

    expect(wish).toEqual(['Приставка', 'Велосипед', 'Гитара'])
  })

  it('состояния вещи в контракте нет — не выдумываем его', () => {
    const item = mapItem(apiItem())

    expect(item.category).toBe('')
    expect(item.condition).toBeUndefined()
  })

  it('описание доезжает до модели — по нему бэкенд и ищет обмен', () => {
    expect(mapItem(apiItem({ offerDescription: 'Рама 19", катался два сезона' })).description).toBe(
      'Рама 19", катался два сезона',
    )
  })

  it('пустое описание — это его отсутствие, а не пустая строка в интерфейсе', () => {
    expect(mapItem(apiItem({ offerDescription: '' })).description).toBeUndefined()
  })

  it('стадия обработки переводится в участие в обмене', () => {
    // Не `idle`: обмен у вещи включён, просто бэкенд ещё разбирает описание.
    expect(mapItem(apiItem({ status: 'ANALYZING' })).status).toBe('analyzing')
    expect(mapItem(apiItem({ status: 'ACTION_REQUIRED' })).status).toBe('needs_category')
    expect(mapItem(apiItem({ status: 'MATCHING' })).status).toBe('searching')
    expect(mapItem(apiItem({ status: 'LOCKED' })).status).toBe('reserved')
    expect(mapItem(apiItem({ status: 'WITHDRAWN' })).status).toBe('withdrawn')
  })

  it('вариант без раздела попадает в нерешённые: по нему и спросят человека', () => {
    const item = mapItem(
      apiItem({
        status: 'ACTION_REQUIRED',
        wishes: [
          { id: 1, categoryId: 3, description: 'Игровая приставка' },
          { id: 2, categoryId: null, description: 'Что-нибудь интересное' },
        ],
      }),
    )

    // Формулировки остаются все: спрашивают про раздел, а не про сам вариант желания.
    expect(item.wish).toEqual(['Игровая приставка', 'Что-нибудь интересное'])
    expect(item.pendingWishes).toEqual([{ id: 2, description: 'Что-нибудь интересное' }])
  })

  it('первая картинка становится фото карточки, пустой список — отсутствием фото', () => {
    expect(mapItem(apiItem()).photoUrl).toBe('/mock/items/bike.jpg')
    expect(mapItem(apiItem({ imageUrls: [] })).photoUrl).toBeUndefined()
  })

  it('бэкенд присылает null вместо пустого списка картинок — это не должно ронять список', () => {
    // Контракт объявляет imageUrls обязательным массивом, но у вещи без фото приходит null.
    const withoutImages = { ...apiItem(), imageUrls: null } as unknown as ApiItem

    expect(mapItem(withoutImages).photoUrl).toBeUndefined()
  })
})
