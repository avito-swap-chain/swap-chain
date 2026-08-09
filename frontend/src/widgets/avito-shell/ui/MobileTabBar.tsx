import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { chainKeys, getMyChains, needsMyAction } from '@/entities/chain'
import { asset, cx } from '@/shared/lib'
import { PERSONAS, usePersonaStore } from '@/shared/model/persona'
import { IconBox, IconChat, IconSwap } from '@/shared/ui'
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

  const personaId = usePersonaStore((state) => state.personaId)
  const persona = PERSONAS.find((item) => item.id === personaId) ?? PERSONAS[0]

  const tabClass =
    'flex flex-1 flex-col items-center gap-0.5 pt-1.5 pb-1 text-[11px] leading-4 outline-offset-[-2px] focus-visible:outline-2 focus-visible:outline-brand'

  return (
    <nav
      aria-label="Разделы"
      className="fixed inset-x-0 bottom-0 z-10 flex h-[52px] border-t border-line bg-card lg:hidden"
    >
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
          {waiting > 0 && (
            <span className="absolute -top-1 -right-2 grid min-w-[16px] place-items-center rounded-full bg-accent-red px-1 text-[10px] leading-4 font-bold text-white">
              {waiting}
            </span>
          )}
        </span>
        Обмен
      </Link>

      <Link
        to="/messages"
        aria-current={section === 'messages' ? 'page' : undefined}
        className={cx(tabClass, section === 'messages' ? 'text-ink' : 'text-ink-3')}
      >
        <IconChat size={24} />
        Сообщения
      </Link>

      {/* Профиля нет в MVP — показываем неактивным, как в остальном кабинете. */}
      <span className={cx(tabClass, 'cursor-not-allowed text-ink-3')} title="В демо не открывается">
        <img src={asset(persona.avatarUrl)} alt="" className="size-6 rounded-full object-cover" />
        Профиль
      </span>
    </nav>
  )
}
