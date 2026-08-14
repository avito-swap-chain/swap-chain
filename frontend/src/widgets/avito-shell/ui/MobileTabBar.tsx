import { useQuery } from '@tanstack/react-query'
import { Link, useLocation } from 'react-router-dom'
import { chainKeys, getMyChains, needsMyAction } from '@/entities/chain'
import { asset, cx } from '@/shared/lib'
import { PERSONAS, usePersonaStore } from '@/shared/model/persona'
import { getCurrentUser, sessionKeys } from '@/shared/model/session'
import { Counter, IconBox, IconChat, IconFlag, IconSwap } from '@/shared/ui'
import type { Section } from '../lib/navigation'

/**
 * Нижняя панель — главная навигация мобильного Авито. На телефоне у них нет ни сайдбара,
 * ни ленты разделов сверху: разделы живут в панели у большого пальца, а профиль — такая же
 * вкладка, как остальные. Высота и линия сверху взяты с их мобильной вёрстки.
 *
 * Панель только для узких окон: от `lg` навигация возвращается в левую колонку кабинета.
 */
export function MobileTabBar({ section }: { section: Section }) {
  const { data } = useQuery({ queryKey: chainKeys.my(), queryFn: getMyChains })
  const waiting = data?.filter(needsMyAction).length ?? 0

  const { data: user } = useQuery({ queryKey: sessionKeys.current(), queryFn: getCurrentUser })
  const { pathname } = useLocation()

  const personaId = usePersonaStore((state) => state.personaId)
  const persona = PERSONAS.find((item) => item.id === personaId) ?? PERSONAS[0]

  const tabClass =
    'flex flex-1 flex-col items-center gap-0.5 pt-1.5 pb-1 text-[11px] leading-4 outline-offset-[-2px] focus-visible:outline-2 focus-visible:outline-brand'

  return (
    <nav
      aria-label="Разделы"
      className="fixed inset-x-0 bottom-0 z-10 flex h-[52px] border-t border-line bg-card lg:hidden"
    >
      {/* У сотрудника ПВЗ вместо своих вещей и обменов — его рабочие разделы: за стойкой
          нужны очередь доставок и разбор жалоб, а объявлений у служебного аккаунта нет. */}
      {user?.isAdmin ? (
        <>
          <Link
            to="/admin/deliveries"
            aria-current={pathname === '/admin/deliveries' ? 'page' : undefined}
            className={cx(tabClass, pathname === '/admin/deliveries' ? 'text-ink' : 'text-ink-3')}
          >
            <IconBox size={24} />
            Доставки
          </Link>

          <Link
            to="/admin/moderation"
            aria-current={pathname.startsWith('/admin/moderation') ? 'page' : undefined}
            className={cx(
              tabClass,
              pathname.startsWith('/admin/moderation') ? 'text-ink' : 'text-ink-3',
            )}
          >
            <IconFlag size={24} />
            Жалобы
          </Link>
        </>
      ) : (
        <>
          <Link
            to="/"
            aria-current={section === 'items' ? 'page' : undefined}
            className={cx(tabClass, section === 'items' ? 'text-ink' : 'text-ink-3')}
          >
            <IconBox size={24} />
            Объявления
          </Link>

          <Link
            to="/exchange"
            aria-current={section === 'exchange' ? 'page' : undefined}
            className={cx(tabClass, section === 'exchange' ? 'text-ink' : 'text-ink-3')}
          >
            <span className="relative">
              <IconSwap size={24} />
              {waiting > 0 && <Counter className="absolute -top-1 -right-2">{waiting}</Counter>}
            </span>
            Обмен
          </Link>
        </>
      )}

      <Link
        to="/messages"
        aria-current={section === 'messages' ? 'page' : undefined}
        className={cx(tabClass, section === 'messages' ? 'text-ink' : 'text-ink-3')}
      >
        <IconChat size={24} />
        Сообщения
      </Link>

      {/* Профиль — вход во всё, чего нет в панели: отзывы, чёрный список, правка имени.
          Раньше вкладка была неактивной заглушкой, и на телефоне эти разделы оставались
          недостижимы — меню кабинета там свёрнуто в эту самую панель. */}
      <Link
        to="/profile"
        aria-current={pathname === '/profile' ? 'page' : undefined}
        className={cx(tabClass, pathname === '/profile' ? 'text-ink' : 'text-ink-3')}
      >
        <img src={asset(persona.avatarUrl)} alt="" className="size-6 rounded-full object-cover" />
        Профиль
      </Link>
    </nav>
  )
}
