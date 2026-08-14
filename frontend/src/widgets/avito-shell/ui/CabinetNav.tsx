import { useQuery } from '@tanstack/react-query'
import { Link, useLocation } from 'react-router-dom'
import { chainKeys, getMyChains, needsMyAction } from '@/entities/chain'
import { getThreads, messageKeys } from '@/entities/message'
import { getReports, moderationKeys } from '@/entities/moderation'
import { getNotifications, notificationKeys } from '@/entities/notification'
import { cx } from '@/shared/lib'
import { Counter } from '@/shared/ui'
import { getCurrentUser, sessionKeys } from '@/shared/model/session'
import type { Section } from '../lib/navigation'

const ITEMS_URL = '/'
const EXCHANGE_URL = '/exchange'
const REVIEWS_URL = '/reviews'
const MESSAGES_URL = '/messages'
const NOTIFICATIONS_URL = '/notifications'
const ADMIN_URL = '/admin/deliveries'
const MODERATION_URL = '/admin/moderation'
const AUDIT_URL = '/admin/audit'
const METRICS_URL = '/admin/metrics'

// Одна разметка на оба вида: на узких окнах — чипы в горизонтальной ленте,
// от `lg` — строки вертикального меню слева, как в кабинете Авито: кегль 15 на строке 24,
// шаг между пунктами 4px.
const itemClass =
  'flex shrink-0 items-center rounded-chip px-2.5 py-1.5 text-[13px] font-semibold outline-offset-2 focus-visible:outline-2 focus-visible:outline-brand lg:mb-1 lg:w-full lg:rounded-none lg:px-0 lg:py-0 lg:text-[15px] lg:leading-6'

/* В кабинете Авито пункты меню — обычные голубые ссылки, а текущий раздел набран
 * чёрным жирным без подложки. Заливка остаётся только чипам на узких экранах. */
const restClass = 'text-ink-2 hover:bg-line-2 lg:bg-transparent lg:font-normal lg:text-brand-hover'
const currentClass = 'bg-line-2 text-ink lg:bg-transparent lg:font-bold lg:text-ink'

/* Группа пунктов. На узких окнах обёртки нет вовсе (`contents`) — там лента чипов;
 * от `lg` группы разделены линией, как разделы меню в их кабинете. */
const groupClass = 'contents lg:block lg:border-t lg:border-line lg:py-3 lg:first:border-0'

/**
 * Меню личного кабинета — единственный верхний уровень навигации.
 * «Обмен» с бейджем — точка входа в сервис: не витрина, а раздел,
 * куда зовут ровно тогда, когда ход за пользователем.
 */
export function CabinetNav({ section, className }: { section: Section; className?: string }) {
  const { data } = useQuery({ queryKey: chainKeys.my(), queryFn: getMyChains })
  const waiting = data?.filter(needsMyAction).length ?? 0

  const { data: user } = useQuery({ queryKey: sessionKeys.current(), queryFn: getCurrentUser })

  // Счётчики берутся тем же ключом, что и значки в шапке: данные уже в кэше, лишнего
  // запроса не будет. Без них пункт меню молчал о непрочитанном, хотя значок сверху уже
  // горел — и человек не понимал, куда идти.
  const { data: unread = 0 } = useQuery({
    queryKey: messageKeys.list(),
    queryFn: getThreads,
    select: (list) => list.totalUnread,
  })
  const { data: news = 0 } = useQuery({
    queryKey: notificationKeys.list(),
    queryFn: getNotifications,
    select: (list) => list.totalUnread,
  })

  // Сколько жалоб ждёт разбора. Спрашиваем только у модератора: остальным ручка ответит 403,
  // а счётчик в меню — единственное место, где видно, что в очереди появилось новое.
  const { data: openReports = [] } = useQuery({
    queryKey: moderationKeys.reports({ status: 'open' }),
    queryFn: () => getReports({ status: 'open' }),
    enabled: Boolean(user?.isAdmin),
    // Жалоба приходит от другого человека: без опроса очередь у открытой админки молчала бы.
    refetchInterval: 15_000,
  })

  const isExchange = section === 'exchange'

  // Подсветка держится на разделе целиком (новое объявление и включение обмена — часть
  // объявлений), а current достаётся только пункту, который ведёт ровно на открытую страницу.
  const { pathname } = useLocation()

  // Админка — не раздел кабинета: её нет ни в `Section`, ни в нижней панели, поэтому
  // оболочка считает её объявлениями. Разбираем по пути, чтобы жирным был открытый пункт.
  // Разбор одной жалобы — часть модерации, поэтому пункт подсвечен и на вложенном экране.
  const isDeliveries = pathname === ADMIN_URL
  const isModeration = pathname.startsWith(MODERATION_URL)
  const isAudit = pathname === AUDIT_URL
  const isMetrics = pathname === METRICS_URL
  const isAdmin = isDeliveries || isModeration || isAudit || isMetrics

  return (
    <nav
      aria-label="Личный кабинет"
      className={cx(
        'no-scrollbar flex gap-1 overflow-x-auto px-3 py-1.5 max-lg:border-b max-lg:border-line-2 sm:py-2 lg:flex-col lg:gap-0 lg:overflow-visible lg:p-0',
        className,
      )}
    >
      {/* У сотрудника ПВЗ своих вещей в кабинете нет: он за стойкой, и объявления,
          обмен и отзывы ему только мешают найти нужный раздел. */}
      {!user?.isAdmin && (
        <div className={groupClass}>
          <Link
            to={ITEMS_URL}
            aria-current={pathname === ITEMS_URL ? 'page' : undefined}
            className={cx(itemClass, section === 'items' && !isAdmin ? currentClass : restClass)}
          >
            Мои объявления
          </Link>

          <Link
            to={EXCHANGE_URL}
            aria-current={pathname === EXCHANGE_URL ? 'page' : undefined}
            className={cx(itemClass, 'gap-2', isExchange ? currentClass : restClass)}
          >
            Обмен
            {waiting > 0 && <Counter title="Ждут вашего действия">{waiting}</Counter>}
          </Link>
        </div>
      )}

      <div className={groupClass}>
        {/* Отзывы у Авито — отдельный пункт меню, а не вкладка профиля: рейтинг смотрят
            чаще, чем сам профиль, а в обмене он ещё и единственный довод о человеке. */}
        {!user?.isAdmin && (
          <Link
            to={REVIEWS_URL}
            aria-current={pathname === REVIEWS_URL ? 'page' : undefined}
            className={cx(itemClass, section === 'reviews' ? currentClass : restClass)}
          >
            Мои отзывы
          </Link>
        )}

        <Link
          to={MESSAGES_URL}
          aria-current={pathname === MESSAGES_URL ? 'page' : undefined}
          className={cx(itemClass, 'gap-2', section === 'messages' ? currentClass : restClass)}
        >
          Сообщения
          {unread > 0 && <Counter title="Непрочитанные сообщения">{unread}</Counter>}
        </Link>

        <Link
          to={NOTIFICATIONS_URL}
          aria-current={pathname === NOTIFICATIONS_URL ? 'page' : undefined}
          className={cx(itemClass, 'gap-2', section === 'notifications' ? currentClass : restClass)}
        >
          Уведомления
          {news > 0 && <Counter title="Новые уведомления">{news}</Counter>}
        </Link>
      </div>

      {/* Рабочее место сотрудника ПВЗ. Обычному пользователю показывать его нечего:
          в списке чужие вещи и чужие имена, а кнопки всё равно вернут 403. */}
      {user?.isAdmin && (
        <div className={groupClass}>
          <Link
            to={ADMIN_URL}
            aria-current={isDeliveries ? 'page' : undefined}
            className={cx(itemClass, isDeliveries ? currentClass : restClass)}
          >
            Доставки ПВЗ
          </Link>

          <Link
            to={MODERATION_URL}
            aria-current={isModeration ? 'page' : undefined}
            className={cx(itemClass, 'gap-2', isModeration ? currentClass : restClass)}
          >
            Жалобы
            {openReports.length > 0 && (
              <Counter title="Жалобы ждут разбора">{openReports.length}</Counter>
            )}
          </Link>

          <Link
            to={AUDIT_URL}
            aria-current={isAudit ? 'page' : undefined}
            className={cx(itemClass, isAudit ? currentClass : restClass)}
          >
            Журнал
          </Link>

          <Link
            to={METRICS_URL}
            aria-current={isMetrics ? 'page' : undefined}
            className={cx(itemClass, isMetrics ? currentClass : restClass)}
          >
            Воронка
          </Link>
        </div>
      )}
    </nav>
  )
}
