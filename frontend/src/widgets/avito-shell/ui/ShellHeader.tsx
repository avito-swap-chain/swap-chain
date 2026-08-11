import { useQuery } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { Link } from 'react-router-dom'
import { getThreads, messageKeys } from '@/entities/message'
import { isBackendConnected } from '@/shared/config/backend'
import { getNotifications, notificationKeys } from '@/entities/notification'
import { SwitchPersona } from '@/features/switch-persona'
import { BrandMark, IconBell, IconChat, IconHeart, IconPlus } from '@/shared/ui'

/** Сервисные ссылки верхнего яруса — в кабинете Авито они на месте, но никуда не ведут из демо. */
const SERVICE_LINKS = ['Для бизнеса', 'Помощь', 'Каталоги', '#яПомогаю']

/** Рубрикатор второго яруса. Тоже декорация: обмен живёт в кабинете, а не в каталоге. */
const CATEGORIES = ['Авто', 'Недвижимость', 'Работа', 'Услуги', 'Ещё']

/**
 * Шапка кабинета Авито — два яруса: сверху сервисные ссылки и действия аккаунта,
 * снизу лок-ап и рубрикатор. Поиска здесь нет намеренно: на страницах личного кабинета
 * Авито его не показывает — кабинет про свои вещи, а не про чужие.
 *
 * Всё, кроме «Разместить объявление», «Мои объявления» и переключателя персон, — декорация:
 * она задаёт контекст «мы внутри Авито», без которого раздел читается отдельным продуктом.
 * На узких окнах ярусы схлопываются в один: там ценен каждый пиксель высоты.
 */
/**
 * Иконка-действие со счётчиком. Размеры с шапки Авито: иконка 28px, счётчик — красный
 * кружок 15px с числом кеглем 13 обычного начертания, приподнятый на угол иконки.
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
      className="relative rounded-sm outline-offset-4 hover:text-ink focus-visible:outline-2 focus-visible:outline-brand"
    >
      {children}
      {count > 0 && (
        <span className="absolute -top-1 -right-1.5 grid size-[15px] place-items-center rounded-full bg-accent-red text-[13px] leading-none text-white">
          {count}
        </span>
      )}
    </Link>
  )
}

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

  return (
    <header className="border-b border-line-2 bg-card">
      <div className="mx-auto w-full max-w-page px-3 sm:px-4 lg:px-6">
        <div className="flex items-center gap-5 py-2 max-lg:hidden">
          {/* Кегли ярусов — как у Авито: верхняя строка 15px, рубрикатор ниже 13px. */}
          <nav aria-hidden className="flex items-center gap-5 text-[15px] text-ink-2">
            {SERVICE_LINKS.map((label) => (
              <span key={label} className="cursor-default">
                {label}
              </span>
            ))}
          </nav>

          <div className="ml-auto flex items-center gap-5 text-[13px]">
            <Link
              to="/items/new"
              className="flex items-center gap-1.5 rounded-sm font-semibold outline-offset-4 hover:text-ink-2 focus-visible:outline-2 focus-visible:outline-brand"
            >
              <IconPlus size={13} />
              Разместить объявление
            </Link>
            <Link
              to="/"
              className="rounded-sm font-semibold outline-offset-4 hover:text-ink-2 focus-visible:outline-2 focus-visible:outline-brand"
            >
              Мои объявления
            </Link>

            <span className="flex items-center gap-4 text-ink-2">
              <span aria-hidden title="В демо не открывается">
                <IconHeart size={28} />
              </span>

              <IconAction to="/notifications" label="Уведомления" count={news}>
                <IconBell size={28} />
              </IconAction>

              <IconAction to="/messages" label="Сообщения" count={unread}>
                <IconChat size={28} />
              </IconAction>
            </span>

            <SwitchPersona />
          </div>
        </div>

        <div className="flex items-center gap-2 py-2 sm:gap-2.5 lg:gap-8 lg:py-3">
          <Link
            to="/"
            className="shrink-0 rounded-sm outline-offset-4 focus-visible:outline-2 focus-visible:outline-brand"
          >
            <BrandMark />
          </Link>

          <nav aria-hidden className="flex items-center gap-6 text-[13px] max-lg:hidden">
            {CATEGORIES.map((label) => (
              <span key={label} className="cursor-default">
                {label}
              </span>
            ))}
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
