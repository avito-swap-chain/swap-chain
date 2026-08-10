import { describe, expect, it } from 'vitest'
import type { Item as ApiItem } from '@/shared/api/generated/model'
import { mapItem } from './mapItem'

const apiItem = (over: Partial<ApiItem> = {}): ApiItem => ({
  id: 7,
  userId: 1,
  offerTitle: 'Горный велосипед',
  offerDescription: 'Спорт и отдых, хорошее',
  wantDescription: 'Игровая приставка',
  imageUrls: ['/mock/items/bike.jpg'],
  status: 'MATCHING',
  createdAt: '2026-08-09T10:00:00Z',
  updatedAt: '2026-08-09T10:00:00Z',
  ...over,
})

describe('mapItem — вещь из контракта в нашу модель', () => {
  it('желание одной строкой разворачивается в вариант', () => {
    expect(mapItem(apiItem()).wish).toEqual([{ category: '', description: 'Игровая приставка' }])
  })

  it('пустое желание — это отсутствие вариантов, а не вариант с пустым текстом', () => {
    expect(mapItem(apiItem({ wantDescription: '' })).wish).toEqual([])
  })

  it('категории и состояния в контракте нет — не выдумываем их', () => {
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
    expect(mapItem(apiItem({ status: 'MATCHING' })).status).toBe('searching')
    expect(mapItem(apiItem({ status: 'LOCKED' })).status).toBe('reserved')
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
