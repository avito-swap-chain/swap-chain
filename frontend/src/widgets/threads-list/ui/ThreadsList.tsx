import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getThreads, isServiceThread, messageKeys, ThreadCard } from '@/entities/message'
import { cx } from '@/shared/lib'
import { IconSearch, Notice } from '@/shared/ui'

/** Фильтры над списком. У Авито их три, но «важных» у нас нет — выдумывать признак незачем. */
const FILTERS = [
  { key: 'all', label: 'Все' },
  { key: 'unread', label: 'Непрочитанные' },
] as const

type Filter = (typeof FILTERS)[number]['key']

const norm = (text: string) => text.toLowerCase().trim()

/**
 * Список переписок. Разговоры заводятся у предложения обмена — сюда они попадают сами,
 * поэтому «пустого» состояния с призывом написать первым здесь нет: писать наугад,
 * не видя варианта обмена, всё равно некому.
 *
 * Сверху поиск и фильтры, как в мессенджере Авито. Поиск идёт по собеседнику, вещи
 * и последней реплике — по тому, что человек и помнит о разговоре.
 */
export function ThreadsList() {
  const [query, setQuery] = useState('')
  const [filter, setFilter] = useState<Filter>('all')

  const { data, isPending, isError } = useQuery({
    queryKey: messageKeys.list(),
    queryFn: getThreads,
  })

  if (isPending) return <Notice>Загрузка…</Notice>
  if (isError) return <Notice tone="error">Не удалось загрузить сообщения</Notice>

  const search = norm(query)
  const found = data.threads.filter((thread) => {
    if (filter === 'unread' && thread.unreadCount === 0) return false
    if (!search) return true

    return [thread.peerName, thread.itemTitle, thread.lastMessage?.text]
      .filter((field): field is string => Boolean(field))
      .some((field) => norm(field).includes(search))
  })

  // Канал сервиса держим первым всегда: он не участвует в сортировке по времени.
  const threads = [...found].sort((a, b) => Number(isServiceThread(b)) - Number(isServiceThread(a)))

  return (
    <div className="flex flex-col gap-3">
      <label className="flex items-center gap-2.5 rounded-input bg-line-2 px-3.5 py-2.5 text-ink-3 focus-within:outline-2 focus-within:outline-brand">
        <IconSearch size={18} />
        <input
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          placeholder="Поиск по сообщениям"
          aria-label="Поиск по сообщениям"
          className="w-full bg-transparent font-sans text-[15px] leading-5 text-ink placeholder:text-ink-3 focus:outline-none"
        />
      </label>

      <div className="flex gap-2">
        {FILTERS.map((option) => (
          <button
            key={option.key}
            type="button"
            onClick={() => setFilter(option.key)}
            className={cx(
              'cursor-pointer rounded-chip px-3.5 py-2 font-sans text-[13.5px] leading-4 font-semibold outline-offset-2 transition-colors focus-visible:outline-2 focus-visible:outline-brand',
              filter === option.key ? 'bg-dark text-white' : 'bg-line-2 text-ink hover:bg-line',
            )}
          >
            {option.label}
          </button>
        ))}
      </div>

      {threads.length === 0 ? (
        <p className="py-6 text-center text-[13.5px] text-ink-2">
          {filter === 'unread' ? 'Непрочитанных сообщений нет' : 'Ничего не нашлось'}
        </p>
      ) : (
        <div className="flex flex-col">
          {threads.map((thread) => (
            <ThreadCard key={`${thread.chainId}:${thread.counterpartId}`} thread={thread} />
          ))}
        </div>
      )}
    </div>
  )
}
