import { useQuery } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { Link } from 'react-router-dom'
import { getThreads, messageKeys } from '@/entities/message'
import { isBackendConnected } from '@/shared/config/backend'
import { getNotifications, notificationKeys } from '@/entities/notification'
import { SwitchPersona } from '@/features/switch-persona'
import {
  BrandMark,
  IconAdd,
  IconBusiness,
  IconCart,
  IconExpandMore,
  IconFavorites,
  IconItems,
  IconMessages,
  IconNotifications,
} from '@/shared/ui'

/**
 * Сервисные ссылки верхнего яруса. Это настоящие адреса Авито и открываются они на Авито:
 * раздел обмена живёт внутри их кабинета, и уводить оттуда некуда — своей «Помощи»
 * или «Каталогов» у нас нет и выдумывать их значило бы обещать несуществующее.
 */
const CAREER_URL = 'https://career.avito.com/'
const SUPPORT_URL = 'https://support.avito.ru'
const CARE_URL = 'https://www.avito.ru/avito-care'
const FAVORITES_URL = 'https://www.avito.ru/favorites'
const CART_URL = 'https://www.avito.ru/order/cart'

const BUSINESS_MENU = [
  { label: 'Продавать', href: 'https://www.avito.ru/business' },
  { label: 'Покупать', href: 'https://www.avito.ru/all/business360' },
  { label: 'Нанимать', href: 'https://www.avito.ru/employer' },
]

const CATALOGS_MENU = [{ label: 'Каталог автомобилей', href: 'https://www.avito.ru/catalog/auto' }]

/** Рубрикатор второго яруса — тоже настоящий: обмен живёт в кабинете, а не в каталоге. */
const CATEGORIES = [
  { label: 'Бизнес360', href: 'https://www.avito.ru/all/business360' },
  { label: 'Авто', href: 'https://www.avito.ru/all/transport' },
  { label: 'Недвижимость', href: 'https://www.avito.ru/all/nedvizhimost' },
  { label: 'Работа', href: 'https://www.avito.ru/all/rabota' },
  { label: 'Услуги', href: 'https://www.avito.ru/all/predlozheniya_uslug' },
]

const MORE_MENU = [
  { label: 'Личные вещи', href: 'https://www.avito.ru/all/lichnye_veschi' },
  { label: 'Для дома и дачи', href: 'https://www.avito.ru/all/dlya_doma_i_dachi' },
  { label: 'Электроника', href: 'https://www.avito.ru/all/bytovaya_elektronika' },
  { label: 'Хобби и отдых', href: 'https://www.avito.ru/all/hobbi_i_otdyh' },
  { label: 'Животные', href: 'https://www.avito.ru/all/zhivotnye' },
  { label: 'Для бизнеса', href: 'https://www.avito.ru/all/dlya_biznesa' },
]

/** Ссылка на Авито. Открывается новой вкладкой: кабинет обмена остаётся на месте. */
function ExternalLink({
  href,
  className,
  children,
}: {
  href: string
  className: string
  children: ReactNode
}) {
  return (
    <a href={href} target="_blank" rel="noreferrer noopener" className={className}>
      {children}
    </a>
  )
}

/**
 * Пункт с выпадающим списком. У Авито он раскрывается наведением, поэтому и здесь
 * это hover, а не клик; клавиатуре хватает `focus-within` — меню раскрывается,
 * когда фокус доходит до самой кнопки, и держится, пока идёт по её ссылкам.
 *
 * Карточка меню — их же: белая, радиус 24, тень всплытия, пункты 15/22 чёрным.
 */
function HeaderMenu({
  label,
  items,
  className,
  chevron = true,
  children,
}: {
  label: string
  items: { label: string; href: string }[]
  className: string
  /** У рубрикатора «Ещё» шеврона нет — как и у Авито: там меню открывает само слово. */
  chevron?: boolean
  children?: ReactNode
}) {
  return (
    <div className="group relative flex">
      <button type="button" aria-haspopup="menu" className={className}>
        {children}
        {label}
        {chevron && <IconExpandMore size={20} className="ml-0.5" />}
      </button>

      <div className="invisible absolute top-full left-0 z-20 pt-1.5 opacity-0 group-focus-within:visible group-focus-within:opacity-100 group-hover:visible group-hover:opacity-100">
        <div className="w-max min-w-[158px] rounded-[24px] bg-card py-3 shadow-pop">
          {items.map((item) => (
            <ExternalLink
              key={item.href}
              href={item.href}
              className="block px-6 pt-2 pb-2.5 text-[15px] leading-[22px] text-ink hover:text-brand"
            >
              {item.label}
            </ExternalLink>
          ))}
        </div>
      </div>
    </div>
  )
}

/**
 * Иконка-действие со счётчиком. Размеры с шапки Авито: иконка 28px, счётчик — красный
 * кружок 15px с числом кеглем 11 обычного начертания, приподнятый на угол иконки.
 */
function IconAction({
  to,
  label,
  count,
  children,
}: {
  to: string
  label: string
  count: number
  children: ReactNode
}) {
  return (
    <Link
      to={to}
      aria-label={count > 0 ? `${label}: ${count}` : label}
      title={label}
      className="relative flex px-[11px] pt-4 pb-2 text-ink-4 outline-offset-[-4px] hover:text-ink-3 focus-visible:outline-2 focus-visible:outline-brand"
    >
      {children}
      {count > 0 && (
        <span className="absolute top-[15px] right-1.5 grid size-[15px] place-items-center rounded-full bg-accent-red text-[11px] leading-none text-white">
          {count}
        </span>
      )}
    </Link>
  )
}

/**
 * Шапка кабинета Авито — два яруса: сверху сервисные ссылки и действия аккаунта,
 * снизу лок-ап и рубрикатор. Поиска здесь нет намеренно: на страницах личного кабинета
 * Авито его не показывает — кабинет про свои вещи, а не про чужие.
 *
 * Всё, кроме «Разместить объявление», «Мои объявления» и переключателя персон, ведёт
 * на настоящий Авито: она задаёт контекст «мы внутри Авито», без которого раздел читается
 * отдельным продуктом. На узких окнах ярусы схлопываются в один: там ценен каждый пиксель.
 */
export function ShellHeader() {
  // Отдельной ручки счётчика в контракте нет: непрочитанное приходит вместе со списком
  // переписок, и обновляет его тот же периодический запрос — так задумано на бэкенде.
  const { data: unread = 0 } = useQuery({
    queryKey: messageKeys.list(),
    queryFn: getThreads,
    select: (list) => list.totalUnread,
    // Ошибка гасит опрос: если ручки нет или бэкенд лёг, счётчик в шапке не должен
    // долбить её раз в десять секунд до конца сессии.
    refetchInterval: (query) => (query.state.error ? false : 10_000),
  })
  const { data: news = 0 } = useQuery({
    queryKey: notificationKeys.list(),
    queryFn: getNotifications,
    select: (list) => list.totalUnread,
    // На моках уведомления рождаются по таймерам (цепочка собралась, обмен завершён),
    // и без периодической проверки счётчик ожил бы только при переходе по разделам.
    // С бэкендом их приносит событие потока, поэтому опрос там не нужен.
    refetchInterval: isBackendConnected ? false : 2000,
  })

  // Кегли и высоты ярусов — с их шапки: верхний ярус 54px и кегль 15/22, рубрикатор 48px.
  const serviceClass =
    'flex items-center rounded-sm px-2 text-[15px] leading-[22px] text-ink-2 outline-offset-2 hover:text-ink focus-visible:outline-2 focus-visible:outline-brand'
  const actionClass =
    'flex items-center gap-[5px] rounded-sm px-2 pt-4 pb-2 text-[15px] leading-[22px] outline-offset-[-4px] hover:text-brand focus-visible:outline-2 focus-visible:outline-brand'
  const categoryClass =
    'rounded-sm px-2.5 text-[15px] leading-[22px] outline-offset-2 hover:text-brand focus-visible:outline-2 focus-visible:outline-brand'

  return (
    <header className="border-b border-line-2 bg-card">
      <div className="mx-auto w-full max-w-page px-3 sm:px-4 lg:px-6">
        <div className="flex h-[54px] items-center max-lg:hidden">
          <nav className="-mx-2 flex items-center">
            <HeaderMenu label="Для бизнеса" items={BUSINESS_MENU} className={serviceClass}>
              <IconBusiness size={20} className="mr-[5px]" />
            </HeaderMenu>
            <ExternalLink href={CAREER_URL} className={serviceClass}>
              Карьера в Авито
            </ExternalLink>
            <ExternalLink href={SUPPORT_URL} className={serviceClass}>
              Помощь
            </ExternalLink>
            <HeaderMenu label="Каталоги" items={CATALOGS_MENU} className={serviceClass} />
            <ExternalLink href={CARE_URL} className={serviceClass}>
              #яПомогаю
            </ExternalLink>
          </nav>

          <div className="ml-auto flex items-center">
            <Link to="/items/new" className={actionClass}>
              <IconAdd size={20} />
              Разместить объявление
            </Link>
            <Link to="/" className={actionClass}>
              <IconItems size={20} />
              Мои объявления
            </Link>

            {/* Избранное и корзина в раздел обмена не заходят — они ведут на Авито,
                где у человека и лежит и то и другое. */}
            <ExternalLink
              href={FAVORITES_URL}
              className="flex px-[11px] pt-4 pb-2 text-ink-4 hover:text-ink-3"
            >
              <span className="sr-only">Избранное</span>
              <IconFavorites size={28} />
            </ExternalLink>

            <IconAction to="/notifications" label="Уведомления" count={news}>
              <IconNotifications size={28} />
            </IconAction>

            <IconAction to="/messages" label="Сообщения" count={unread}>
              <IconMessages size={28} />
            </IconAction>

            <ExternalLink
              href={CART_URL}
              className="flex px-[11px] pt-4 pb-2 text-ink-4 hover:text-ink-3"
            >
              <span className="sr-only">Корзина</span>
              <IconCart size={28} />
            </ExternalLink>

            <SwitchPersona />
          </div>
        </div>

        <div className="flex items-center gap-2 py-2 sm:gap-2.5 lg:h-12 lg:gap-8 lg:py-0">
          <Link
            to="/"
            className="shrink-0 rounded-sm outline-offset-4 focus-visible:outline-2 focus-visible:outline-brand"
          >
            <BrandMark />
          </Link>

          <nav className="-mx-2.5 flex items-center max-lg:hidden">
            {CATEGORIES.map((category) => (
              <ExternalLink key={category.href} href={category.href} className={categoryClass}>
                {category.label}
              </ExternalLink>
            ))}
            <HeaderMenu
              label="Ещё"
              items={MORE_MENU}
              chevron={false}
              className={`${categoryClass} flex`}
            />
          </nav>

          {/* На узких окнах ярус один, поэтому переключатель персон переезжает сюда. */}
          <div className="ml-auto shrink-0 lg:hidden">
            <SwitchPersona />
          </div>
        </div>
      </div>
    </header>
  )
}
