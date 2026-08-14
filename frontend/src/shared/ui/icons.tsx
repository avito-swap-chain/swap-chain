import type { ReactNode } from 'react'

interface IconProps {
  size?: number
  className?: string
}

function Icon({
  size = 16,
  strokeWidth = 2,
  className,
  children,
}: IconProps & { strokeWidth?: number; children: ReactNode }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={strokeWidth}
      strokeLinecap="round"
      strokeLinejoin="round"
      className={className}
      aria-hidden="true"
    >
      {children}
    </svg>
  )
}

export const IconCheck = (props: IconProps) => (
  <Icon strokeWidth={3} {...props}>
    <path d="M5 12l5 5 9-11" />
  </Icon>
)

export const IconClock = (props: IconProps) => (
  <Icon strokeWidth={2.2} {...props}>
    <circle cx="12" cy="12" r="8" />
    <path d="M12 8v4l3 2" />
  </Icon>
)

export const IconClose = (props: IconProps) => (
  <Icon strokeWidth={2.4} {...props}>
    <path d="M6 6l12 12M18 6L6 18" />
  </Icon>
)

export const IconPlus = (props: IconProps) => (
  <Icon strokeWidth={3} {...props}>
    <path d="M12 5v14M5 12h14" />
  </Icon>
)

export const IconArrowRight = (props: IconProps) => (
  <Icon {...props}>
    <path d="M5 12h13M13 6l6 6-6 6" />
  </Icon>
)

export const IconChevronLeft = (props: IconProps) => (
  <Icon {...props}>
    <path d="M15 5l-7 7 7 7" />
  </Icon>
)

export const IconImage = (props: IconProps) => (
  <Icon strokeWidth={1.5} {...props}>
    <rect x="3" y="5" width="18" height="14" rx="2" />
    <circle cx="8.5" cy="10" r="1.5" />
    <path d="M3 16l5-4 4 3 3-2 6 5" />
  </Icon>
)

/** Жалоба: флажок, а не восклицательный знак — знак тревожит, флажок помечает. */
export const IconFlag = (props: IconProps) => (
  <Icon strokeWidth={1.8} {...props}>
    <path d="M6 21V4" />
    <path d="M6 4h11l-2 4 2 4H6" />
  </Icon>
)

/** Блокировка: перечёркнутый круг — тот же знак, что и у запрета везде. */
export const IconBan = (props: IconProps) => (
  <Icon strokeWidth={1.8} {...props}>
    <circle cx="12" cy="12" r="8.5" />
    <path d="M6 18L18 6" />
  </Icon>
)

export const IconSearch = (props: IconProps) => (
  <Icon strokeWidth={2} {...props}>
    <circle cx="11" cy="11" r="7" />
    <path d="M16.5 16.5L21 21" />
  </Icon>
)

/** Правка блока: карандаш ведёт к полю, а не открывает новый экран. */
export const IconPencil = (props: IconProps) => (
  <Icon strokeWidth={1.8} {...props}>
    <path d="M4 20.5h4L20 8.5a2.8 2.8 0 0 0-4-4L4 16.5v4z" />
    <path d="M14.5 6l3.5 3.5" />
  </Icon>
)

/**
 * Работа модели. Заливка, а не обводка: искра рядом с текстом читается как значок ИИ
 * только сплошной — в тонком контуре она превращается в звёздочку сноски.
 */
export const IconSparkle = ({ size = 16, className }: IconProps) => (
  <svg
    width={size}
    height={size}
    viewBox="0 0 24 24"
    fill="currentColor"
    className={className}
    aria-hidden="true"
  >
    <path d="M12 1.5c.7 6.2 3.8 9.3 10 10-6.2.7-9.3 3.8-10 10-.7-6.2-3.8-9.3-10-10 6.2-.7 9.3-3.8 10-10z" />
  </svg>
)

/** Обмен: две встречные стрелки — цикл, а не отправка в одну сторону. */
export const IconSwap = (props: IconProps) => (
  <Icon strokeWidth={1.8} {...props}>
    <path d="M4 8h13l-3.5-3.5M20 16H7l3.5 3.5" />
  </Icon>
)

export const IconChat = (props: IconProps) => (
  <Icon strokeWidth={1.8} {...props}>
    <path d="M20 12a7 7 0 01-9.9 6.4L4 20l1.6-5.5A7 7 0 1120 12z" />
  </Icon>
)

/** Плейсхолдер вещи, пока нет фото. */
export const IconBox = (props: IconProps) => (
  <Icon strokeWidth={1.5} {...props}>
    <path d="M3 8l9-5 9 5v8l-9 5-9-5z" />
    <path d="M3 8l9 5 9-5M12 13v8" />
  </Icon>
)

/* --- Иконки дизайн-системы Авито ----------------------------------------------------
 *
 * Ниже — контуры из их набора `design-system-icons` (те же файлы, что шапка Авито грузит
 * маской из `avito.st`), а не нарисованные по памяти пиктограммы. Разница видна сразу:
 * у них сообщения и избранное залитые, колокольчик без язычка, «плюс» тоньше строки.
 * Пока рядом стояли самодельные, шапка читалась как чужая копия кабинета.
 *
 * Пропорции у иконок разные: строчные (`add`, `items`, `expandmore`) нарисованы в высоту
 * строки 20 и уже её, площадные — в квадрате 20×20. Поэтому размер задаётся высотой,
 * а ширина считается от неё: иначе строчные растянутся.
 */
function AvitoIcon({
  w,
  h = 20,
  size = 20,
  className,
  children,
}: IconProps & { w: number; h?: number; children: ReactNode }) {
  return (
    <svg
      width={(size * w) / h}
      height={size}
      viewBox={`0 0 ${w} ${h}`}
      fill="currentColor"
      className={className}
      aria-hidden="true"
    >
      {children}
    </svg>
  )
}

/** «Разместить объявление» в шапке. */
export const IconAdd = (props: IconProps) => (
  <AvitoIcon w={13} {...props}>
    <path
      fillRule="evenodd"
      clipRule="evenodd"
      d="M5.8 11.4V17h1.4v-5.6h5.6V10H7.2V4.4H5.8V10H.2v1.4z"
    />
  </AvitoIcon>
)

/** «Мои объявления» в шапке: карточка позади карточки. */
export const IconItems = (props: IconProps) => (
  <AvitoIcon w={14} {...props}>
    <path d="m10.95 13.53.82-7.56-5.57-.6-.07.63h-1.4l.12-1.17a1 1 0 0 1 1.1-.9l6.37.7a1 1 0 0 1 .88 1.1l-.9 8.35a1 1 0 0 1-1.1.89l-.8-.1v-1.4z" />
    <path
      fillRule="evenodd"
      clipRule="evenodd"
      d="M8 16V8.4H2.4V16zm.4 1.4a1 1 0 0 0 1-1V8a1 1 0 0 0-1-1H2a1 1 0 0 0-1 1v9.4z"
    />
  </AvitoIcon>
)

/** Шеврон вниз у пунктов с выпадающим меню. */
export const IconExpandMore = (props: IconProps) => (
  <AvitoIcon w={10} {...props}>
    <path
      fillRule="evenodd"
      clipRule="evenodd"
      d="m9.5 10.73-1-1.13L5 12.67 1.5 9.6l-1 1.13L5 14.66z"
    />
  </AvitoIcon>
)

/** Кольца «Авито для бизнеса» — знак у первого пункта верхнего яруса. */
export const IconBusiness = (props: IconProps) => (
  <AvitoIcon w={16} {...props}>
    <path d="M8 2.6a8 8 0 1 1 0 16 8 8 0 0 1 0-16m0 1.48a6.52 6.52 0 1 0 0 13.04A6.52 6.52 0 0 0 8 4.08m0 2.08a4.44 4.44 0 1 1 0 8.88 4.44 4.44 0 0 1 0-8.88m0 1.47a2.97 2.97 0 1 0 0 5.94 2.97 2.97 0 0 0 0-5.94" />
  </AvitoIcon>
)

export const IconFavorites = (props: IconProps) => (
  <AvitoIcon w={20} {...props}>
    <path d="M10.01 3.6a5.15 5.15 0 0 1 7.45 0 5.56 5.56 0 0 1 0 7.68L9.98 19l-7.44-7.69a5.54 5.54 0 0 1 0-7.66 5.1 5.1 0 0 1 7.4 0z" />
  </AvitoIcon>
)

export const IconNotifications = (props: IconProps) => (
  <AvitoIcon w={20} {...props}>
    <path d="M10 2a5.53 5.53 0 0 0-5.53 5.53v5.53l-1.97.8v.78c0 .87.71 1.58 1.58 1.58h3.95a1.97 1.97 0 1 0 3.94 0h3.95c.87 0 1.58-.7 1.58-1.58v-.79l-1.97-.79V7.53A5.53 5.53 0 0 0 10 2" />
  </AvitoIcon>
)

export const IconMessages = (props: IconProps) => (
  <AvitoIcon w={20} {...props}>
    <path d="M10 2.2c-2.67 0-4.88.67-6.42 1.95A6.7 6.7 0 0 0 1.2 9.5c0 2.61 1.15 4.73 3.3 6l.41 2.63a.8.8 0 0 0 1.32.48l2.18-1.89q.76.09 1.59.09c2.67 0 4.88-.67 6.42-1.96A6.7 6.7 0 0 0 18.8 9.5c0-2.18-.81-4.04-2.38-5.35C14.88 2.87 12.67 2.2 10 2.2" />
  </AvitoIcon>
)

export const IconCart = (props: IconProps) => (
  <AvitoIcon w={20} {...props}>
    <path d="M4.75 16a1.75 1.75 0 1 1 0 3.5 1.75 1.75 0 0 1 0-3.5m9 0a1.75 1.75 0 1 1 0 3.5 1.75 1.75 0 0 1 0-3.5M3.35 4H18l-2.35 7.59a3 3 0 0 1-2.59 2.1l-7.79.74a1 1 0 0 1-1.07-.78l-.3-1.34L1 1.41 2.55 1z" />
  </AvitoIcon>
)

/** Шеврон вверх: им сворачивают и разворачивают плавающий мессенджер. */
export const IconArrowUp = (props: IconProps) => (
  <AvitoIcon w={20} {...props}>
    <path fillRule="evenodd" clipRule="evenodd" d="M2.73 12 10 4.73 17.27 12 16 13.27l-6-6-6 6z" />
  </AvitoIcon>
)

/** Смена фотографии профиля — кнопка на углу аватара. */
export const IconCamera = (props: IconProps) => (
  <AvitoIcon w={20} {...props}>
    <path
      fillRule="evenodd"
      clipRule="evenodd"
      d="M2.67 14.67V7.72c0-.86.7-1.55 1.55-1.55 1 0 1.94-.46 2.57-1.23l.51-.65c.32-.4.8-.62 1.3-.62h2.8c.5 0 .98.23 1.3.62l.51.65a3.3 3.3 0 0 0 2.56 1.23c.87 0 1.56.7 1.56 1.55v6.95c0 .92-.74 1.66-1.66 1.66H4.33c-.92 0-1.66-.74-1.66-1.66M6 3.25l-.52.64c-.3.39-.77.61-1.26.61A3.2 3.2 0 0 0 1 7.72v6.95C1 16.5 2.5 18 4.33 18h11.34C17.5 18 19 16.5 19 14.67V7.72a3.23 3.23 0 0 0-3.22-3.22c-.5 0-.96-.22-1.26-.6L14 3.24A3.3 3.3 0 0 0 11.4 2H8.6C7.6 2 6.63 2.46 6 3.25m6.33 7.25a2.33 2.33 0 1 1-4.66 0 2.33 2.33 0 0 1 4.66 0m1.67 0a4 4 0 1 1-8 0 4 4 0 0 1 8 0"
    />
  </AvitoIcon>
)

/** Звезда рейтинга — их же, из набора: у самодельной лучи острее и она выбивалась из строки. */
export const IconStar = (props: IconProps) => (
  <AvitoIcon w={20} {...props}>
    <path d="M9.4 1.63c.24-.5.96-.5 1.2 0l2.21 4.8c.1.21.3.36.52.38l5.26.63c.55.06.78.75.36 1.13l-3.88 3.6a.7.7 0 0 0-.2.6l1.03 5.2c.11.54-.47.97-.96.7l-4.62-2.59a.7.7 0 0 0-.64 0l-4.62 2.58a.66.66 0 0 1-.96-.7l1.03-5.19a.7.7 0 0 0-.2-.6l-3.88-3.6a.66.66 0 0 1 .36-1.13l5.26-.63a.7.7 0 0 0 .52-.37z" />
  </AvitoIcon>
)
