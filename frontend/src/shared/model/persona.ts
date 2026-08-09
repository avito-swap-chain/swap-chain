import { create } from 'zustand'

/** Демо-персона: за кого сейчас смотрят сервис. */
export interface Persona {
  id: string
  name: string
  rating: number
  avatarUrl: string
  /** Сколько отзывов набрано: рядом с рейтингом в кабинете Авито всегда стоит их число. */
  reviews: number
  /** Что персона искала на Авито — запросы, а не объявления. */
  searches: string[]
  /** Названия объявлений, сохранённых в избранное. */
  favorites: string[]
}

/**
 * Все трое — участники первой цепочки, поэтому один и тот же обмен виден тремя парами глаз:
 * кто-то ещё думает, кто-то уже подтвердил. Реестр лежит в shared, потому что его читают
 * и мок-api в entities, и переключатель в features — а entities о features знать не могут.
 *
 * Поиски и избранное — мок вместо профиля пользователя: из них выводятся подсказки желания
 * (`suggestWish`). Часть сигналов намеренно ни во что не попадает («фитнес-браслет»,
 * «графический планшет») — такого объявления ни у кого нет, и подсказкой это стать не должно.
 */
export const PERSONAS: Persona[] = [
  {
    id: 'u1',
    name: 'Даша',
    reviews: 24,
    rating: 4.8,
    avatarUrl: '/mock/avatars/u32.jpg',
    searches: ['умные часы', 'механическая клавиатура', 'фитнес-браслет'],
    favorites: ['PlayStation 4 Slim, 1 ТБ'],
  },
  {
    id: 'u2',
    name: 'Марк',
    reviews: 41,
    rating: 4.9,
    avatarUrl: '/mock/avatars/u12.jpg',
    searches: ['монитор', 'горный велосипед', 'графический планшет'],
    favorites: ['Плёночный фотоаппарат'],
  },
  {
    id: 'u3',
    name: 'Лена',
    reviews: 17,
    rating: 4.7,
    avatarUrl: '/mock/avatars/u47.jpg',
    searches: ['кофеварка', 'умные часы', 'горный велосипед', 'виниловый проигрыватель'],
    favorites: ['Наушники'],
  },
]

interface PersonaState {
  personaId: string
  setPersonaId: (id: string) => void
}

/** Кто сейчас «я». В бою здесь была бы сессия авторизации, в демо — выбор персоны. */
export const usePersonaStore = create<PersonaState>()((set) => ({
  personaId: PERSONAS[0].id,
  setPersonaId: (personaId) => set({ personaId }),
}))

/** Текущая персона вне React — мок-api это обычные функции, а не хуки. */
export const currentPersonaId = (): string => usePersonaStore.getState().personaId
