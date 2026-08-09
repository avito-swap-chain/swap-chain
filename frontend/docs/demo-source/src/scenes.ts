/**
 * Сценарий демо. Телефон в кадре один и стоит на месте — меняются экраны внутри него,
 * как при настоящей навигации. Один элемент списка — один экран продукта.
 */
export interface Scene {
  /** Файл в public/screens — снимается со свежего фронта скриптом shoot.mjs. */
  image: string
  title: string
  caption: string
  /** Длинным экранам нужно больше времени: они прокручиваются внутри телефона. */
  seconds: number
  /**
   * Как экран сменяет предыдущий:
   * - `push` — въезжает справа, предыдущий отходит влево (переход вглубь);
   * - `back` — обратный ход, шаг назад;
   * - `fade` — смена контекста, когда экраны не связаны навигацией.
   */
  transition?: 'push' | 'back' | 'fade'
  /**
   * Акцент: подпись уходит, телефон вырастает во весь кадр. Ставим там, где нужно
   * рассмотреть сам экран, а не читать про него.
   */
  focus?: boolean
  /**
   * Куда пользователь нажимает в конце сцены — доли от видимой области экрана (0..1).
   * Есть у каждой сцены, после которой экран меняется: переход без нажатия выглядит так,
   * будто приложение переключилось само, и путь пользователя перестаёт читаться.
   */
  tap?: { x: number; y: number }
  /**
   * Область акцента в долях экрана. На неё наводится кадр и вокруг неё пульсирует обводка.
   * Приближение без такой метки выглядит беспричинным: непонятно, на что смотреть.
   */
  highlight?: { x: number; y: number; width: number; height: number }
  /**
   * Окно свечения по краям экрана, пока работает распознавание, в секундах от начала сцены.
   * Показываем ровно столько, сколько в приложении реально длится ожидание ответа.
   */
  aiGlow?: { from: number; to: number }
}

export const SCENES: Scene[] = [
  {
    image: '01-items.png',
    title: 'Ваши объявления на Авито',
    caption: 'Обмен живёт здесь же — отдельный сервис заводить не нужно',
    seconds: 4,
    tap: { x: 0.5, y: 0.19 },
    transition: 'fade',
  },
  {
    image: '02-new-item.png',
    title: 'Обычная публикация',
    caption: 'Начинается с фотографии — как всегда на Авито',
    seconds: 3.5,
    tap: { x: 0.5, y: 0.29 },
    transition: 'push',
  },
  {
    image: '03-ai-thinking.png',
    title: 'Фото распознаётся',
    caption: 'Название, категорию и состояние сервис определяет сам',
    seconds: 3.5,
    transition: 'fade',
    aiGlow: { from: 0, to: 3.5 },
  },
  {
    image: '04-ai-done.png',
    title: 'Поля заполнены',
    caption: 'Осталось проверить — любое значение можно поправить',
    seconds: 4.5,
    tap: { x: 0.5, y: 0.94 },
    transition: 'fade',
    aiGlow: { from: 0, to: 0.7 },
  },
  {
    image: '05-enable-barter.png',
    title: 'Что хотите взамен',
    caption: 'Подсказки — из того, что вы искали. Вариантов может быть несколько',
    seconds: 5.5,
    tap: { x: 0.32, y: 0.45 },
    transition: 'push',
    highlight: { x: 0.04, y: 0.36, width: 0.92, height: 0.16 },
  },
  {
    image: '06-offers.png',
    title: 'Несколько предложений сразу',
    caption: 'Сервис подбирает варианты параллельно — лайк или дизлайк по каждому',
    seconds: 8,
    tap: { x: 0.39, y: 0.86 },
    transition: 'fade',
  },
  {
    image: '07-chain-offer.png',
    title: 'Кому отдаёте и от кого получаете',
    caption: 'Сначала ваша выгода, потом схема обмена',
    seconds: 7,
    tap: { x: 0.27, y: 0.93 },
    transition: 'push',
    focus: true,
    highlight: { x: 0.05, y: 0.31, width: 0.9, height: 0.15 },
  },
  {
    image: '08-chain-waiting.png',
    title: 'Согласились — ждём остальных',
    caption: 'Обмен стартует, только когда согласны все. До этого вещь остаётся у вас',
    seconds: 4,
    tap: { x: 0.5, y: 0.2 },
    transition: 'fade',
  },
  {
    image: '09-chain-handoff.png',
    title: 'Передача вещей',
    caption: 'Пошагово: кому передать, что забрать, что отметить',
    seconds: 5,
    tap: { x: 0.5, y: 0.94 },
    transition: 'fade',
    focus: true,
    highlight: { x: 0.05, y: 0.4, width: 0.9, height: 0.24 },
  },
  {
    image: '10-chain-completed.png',
    title: 'Обмен завершён',
    caption: 'Цепочка закрывается, когда получение отметили все участники',
    seconds: 3,
    tap: { x: 0.5, y: 0.2 },
    transition: 'fade',
  },
  {
    image: '11-chain-dissolved.png',
    title: 'Не подошло — отказались',
    caption: 'Это отказ от одного варианта, а не крах: вещь осталась у вас',
    seconds: 4,
    transition: 'back',
  },
]

export const INTRO_SECONDS = 4

export interface Span {
  scene: Scene
  from: number
  to: number
}

/** Границы сцен в кадрах: считаются один раз и используются всеми слоями кадра. */
export function timeline(fps: number): Span[] {
  let from = Math.round(INTRO_SECONDS * fps)

  return SCENES.map((scene) => {
    const span = { scene, from, to: from + Math.round(scene.seconds * fps) }
    from = span.to
    return span
  })
}

export const totalFrames = (fps: number) => timeline(fps).at(-1)!.to
