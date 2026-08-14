import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link, useLocation } from 'react-router-dom'
import { getThreads, isServiceThread, messageKeys, ThreadCard } from '@/entities/message'
import { cx } from '@/shared/lib'
import { IconArrowRight, IconArrowUp } from '@/shared/ui'

/**
 * Плавающий мессенджер в правом нижнем углу — как у Авито: переписка догоняет человека
 * на любой странице кабинета, а не ждёт, пока он сам зайдёт в раздел. Для обмена это
 * важнее, чем для продажи: спросить о вещи можно до ответа на предложение, и ответ
 * приходит, пока человек смотрит цепочку.
 *
 * Свёрнутая панель — только заголовок со счётчиком; развёрнутая показывает те же строки
 * переписок, что и раздел, поэтому список здесь не свой, а тот же `ThreadCard`.
 *
 * На самой странице сообщений панели нет: она перекрывала бы список, который и так открыт.
 * На узких окнах её тоже нет — там разделы живут в нижней панели, как в их приложении.
 */
export function MiniMessenger() {
  const [open, setOpen] = useState(false)
  const { pathname } = useLocation()

  const { data } = useQuery({ queryKey: messageKeys.list(), queryFn: getThreads })

  if (pathname.startsWith('/messages')) return null

  const unread = data?.totalUnread ?? 0
  // Канал сервиса держим первым всегда — как в списке раздела.
  const threads = [...(data?.threads ?? [])].sort(
    (a, b) => Number(isServiceThread(b)) - Number(isServiceThread(a)),
  )

  return (
    /* Геометрия с их виджета: свёрнутый 250×40, развёрнутый 411 в ширину,
       прижат к нижнему краю окна и приподнят тенью, а не рамкой. */
    <div
      className={cx(
        'fixed right-5 bottom-0 z-20 hidden rounded-t-[3px] bg-card shadow-pop max-lg:hidden lg:block',
        open ? 'w-[411px]' : 'w-[250px]',
      )}
    >
      <div className={cx('flex items-center gap-2 pr-2 pl-4', open ? 'h-[74px]' : 'h-10')}>
        <span className="flex items-center gap-1.5 text-[18px] leading-6 font-bold">
          Сообщения
          {unread > 0 && (
            <span className="grid size-4 place-items-center rounded-full bg-accent-red text-[11px] leading-none font-normal text-white">
              {unread}
            </span>
          )}
        </span>

        {open && (
          <Link
            to="/messages"
            title="Открыть раздел «Сообщения»"
            aria-label="Открыть раздел «Сообщения»"
            className="ml-auto grid size-9 place-items-center rounded-full text-ink-2 hover:bg-line-2 hover:text-ink focus-visible:outline-2 focus-visible:outline-brand"
          >
            <IconArrowRight size={20} />
          </Link>
        )}

        <button
          type="button"
          aria-expanded={open}
          onClick={() => setOpen((was) => !was)}
          title={open ? 'Свернуть сообщения' : 'Развернуть сообщения'}
          aria-label={open ? 'Свернуть сообщения' : 'Развернуть сообщения'}
          className={cx(
            'grid size-9 cursor-pointer place-items-center rounded-full hover:bg-line-2 focus-visible:outline-2 focus-visible:outline-brand',
            open ? 'rotate-180' : 'ml-auto',
          )}
        >
          <IconArrowUp size={20} />
        </button>
      </div>

      {open && (
        <div className="h-[487px] overflow-y-auto px-2 pb-2">
          {threads.length === 0 ? (
            <p className="px-2 py-6 text-center text-[13.5px] text-ink-2">Переписок пока нет</p>
          ) : (
            threads.map((thread) => (
              <ThreadCard key={`${thread.itemId}:${thread.counterpartId}`} thread={thread} />
            ))
          )}
        </div>
      )}
    </div>
  )
}
