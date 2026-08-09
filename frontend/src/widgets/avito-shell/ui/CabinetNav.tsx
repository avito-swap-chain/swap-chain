import { useQuery } from '@tanstack/react-query'
import { Link, useLocation } from 'react-router-dom'
import { chainKeys, getMyChains, needsMyAction } from '@/entities/chain'
import { cx } from '@/shared/lib'
import type { Section } from '../lib/navigation'

/** Разделы кабинета, которых нет в MVP: показываем неактивными — ради правдоподобия контекста. */
const DECOR = ['Кошелёк']

const ITEMS_URL = '/'
const EXCHANGE_URL = '/exchange'
const MESSAGES_URL = '/messages'
const NOTIFICATIONS_URL = '/notifications'

// Одна разметка на оба вида: на узких окнах — чипы в горизонтальной ленте,
// от `lg` — строки вертикального меню слева, как в кабинете Авито.
const itemClass =
  'shrink-0 rounded-chip px-2.5 py-1.5 text-[13px] font-semibold outline-offset-2 focus-visible:outline-2 focus-visible:outline-brand lg:w-full lg:rounded-none lg:px-0 lg:py-[7px] lg:text-[15px]'

/* В кабинете Авито пункты меню — обычные голубые ссылки, а текущий раздел набран
 * чёрным жирным без подложки. Заливка остаётся только чипам на узких экранах. */
const restClass = 'text-ink-2 hover:bg-line-2 lg:bg-transparent lg:font-normal lg:text-brand'
const currentClass = 'bg-line-2 text-ink lg:bg-transparent lg:font-bold lg:text-ink'

/**
 * Меню личного кабинета — единственный верхний уровень навигации.
 * «Обмен» с бейджем — точка входа в сервис: не витрина, а раздел,
 * куда зовут ровно тогда, когда ход за пользователем.
 */
export function CabinetNav({ section, className }: { section: Section; className?: string }) {
  const { data } = useQuery({ queryKey: chainKeys.my(), queryFn: getMyChains })
  const waiting = data?.filter(needsMyAction).length ?? 0

  const isExchange = section === 'exchange'

  // Подсветка держится на разделе целиком (новое объявление и включение обмена — часть
  // объявлений), а current достаётся только пункту, который ведёт ровно на открытую страницу.
  const { pathname } = useLocation()

  return (
    <nav
      aria-label="Личный кабинет"
      className={cx(
        'no-scrollbar flex gap-1 overflow-x-auto px-3 py-1.5 max-lg:border-b max-lg:border-line-2 sm:py-2 lg:flex-col lg:gap-0 lg:overflow-visible lg:px-0 lg:pt-4 lg:pb-0',
        className,
      )}
    >
      <Link
        to={ITEMS_URL}
        aria-current={pathname === ITEMS_URL ? 'page' : undefined}
        className={cx(itemClass, isExchange ? restClass : currentClass)}
      >
        Мои объявления
      </Link>

      <Link
        to={EXCHANGE_URL}
        aria-current={pathname === EXCHANGE_URL ? 'page' : undefined}
        className={cx(
          itemClass,
          'flex items-center gap-1.5',
          isExchange ? currentClass : restClass,
        )}
      >
        Обмен
        {waiting > 0 && (
          <span
            title="Ждут вашего действия"
            /* Счётчики у Авито красные — азур в их системе означает ссылку, а не тревогу. */
            className="grid min-w-[18px] place-items-center rounded-full bg-accent-red px-1 text-[10.5px] leading-[18px] font-bold text-white lg:ml-auto"
          >
            {waiting}
          </span>
        )}
      </Link>

      <Link
        to={MESSAGES_URL}
        aria-current={pathname === MESSAGES_URL ? 'page' : undefined}
        className={cx(itemClass, section === 'messages' ? currentClass : restClass)}
      >
        Сообщения
      </Link>

      <Link
        to={NOTIFICATIONS_URL}
        aria-current={pathname === NOTIFICATIONS_URL ? 'page' : undefined}
        className={cx(itemClass, section === 'notifications' ? currentClass : restClass)}
      >
        Уведомления
      </Link>

      {DECOR.map((label) => (
        <span
          key={label}
          title="В демо раздел не открывается"
          className={cx(itemClass, 'cursor-not-allowed text-ink-3')}
        >
          {label}
        </span>
      ))}
    </nav>
  )
}
