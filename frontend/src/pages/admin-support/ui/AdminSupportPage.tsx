import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  getSupportQueue,
  getSupportQueueMessages,
  joinSupport,
  leaveSupport,
  replySupport,
  supportKeys,
} from '@/entities/moderation'
import type { SupportThread } from '@/shared/api/generated/model'
import { formatDateTime } from '@/shared/lib'
import { ActionError, Button, EmptyState, Input, Notice, Screen } from '@/shared/ui'

/**
 * Поддержка со стороны сотрудника: очередь вопросов и разговор с человеком.
 *
 * Отвечать можно только подключившись к треду — так решил бэкенд, и это видно в интерфейсе:
 * пока не подключился, форма закрыта, а в списке видно, кто уже взял разговор. Иначе
 * в одном чате оказались бы несколько сотрудников, не видя друг друга.
 */
/** Тред, в котором человек хоть что-то написал: остальные — только приветствие сервиса. */
const hasQuestion = (thread: SupportThread) => thread.lastMessage?.senderType !== 'SYSTEM'

export function AdminSupportPage() {
  const [openId, setOpenId] = useState<number>()
  const [showAll, setShowAll] = useState(false)

  const { data, isPending, isError } = useQuery({
    queryKey: supportKeys.threads(),
    queryFn: getSupportQueue,
    // Вопрос приходит от человека, а не от сотрудника: без опроса очередь застынет.
    refetchInterval: (query) => (query.state.error ? false : 15_000),
  })

  const open = data?.find((thread) => thread.id === openId)

  // Тред заводится каждому вошедшему, и очередь на стенде — это сотни приветствий, среди
  // которых теряются настоящие вопросы. Поэтому по умолчанию показываем те, где человек
  // написал сам; остальные — за переключателем, а не выброшены.
  const asked = (data ?? []).filter(hasQuestion)
  const shown = showAll ? (data ?? []) : asked

  return (
    <Screen width="wide">
      <div className="flex flex-col gap-3.5 p-4">
        <h1 className="text-[22px] leading-7 font-bold lg:text-[32px] lg:leading-10">Поддержка</h1>

        {isPending && <Notice>Загрузка…</Notice>}
        {isError && (
          <Notice tone="error">
            Не удалось загрузить очередь. Раздел открыт только сотрудникам.
          </Notice>
        )}

        {data && data.length > 0 && (
          <div className="flex gap-2">
            <Button variant={showAll ? 'ghost' : 'primary'} onClick={() => setShowAll(false)}>
              С вопросами · {asked.length}
            </Button>
            <Button variant={showAll ? 'primary' : 'ghost'} onClick={() => setShowAll(true)}>
              Все · {data.length}
            </Button>
          </div>
        )}

        {data && shown.length === 0 && (
          <EmptyState
            title="Вопросов нет"
            description="Здесь появятся обращения людей: в обмене спрашивают, когда не понимают, что будет с их вещью."
          />
        )}

        {shown.length > 0 && (
          <ul className="flex flex-col">
            {shown.map((thread) => (
              <li key={thread.id}>
                <QueueRow
                  thread={thread}
                  open={thread.id === openId}
                  onToggle={() => setOpenId(thread.id === openId ? undefined : thread.id)}
                />
              </li>
            ))}
          </ul>
        )}

        {open && <Conversation thread={open} />}
      </div>
    </Screen>
  )
}

function QueueRow({
  thread,
  open,
  onToggle,
}: {
  thread: SupportThread
  open: boolean
  onToggle: () => void
}) {
  return (
    <button
      type="button"
      onClick={onToggle}
      className="flex w-full cursor-pointer flex-col gap-1 border-b border-line py-3.5 text-left last:border-0 sm:flex-row sm:items-center sm:justify-between"
    >
      <span className="min-w-0">
        <span className="block text-[15px] leading-5 font-bold">{thread.user.username}</span>
        <span className="mt-0.5 block truncate text-[13px] leading-4 text-ink-2">
          {thread.lastMessage?.text ?? 'Пока без сообщений'}
        </span>
      </span>

      <span className="flex items-center gap-3 text-[13px] leading-4 text-ink-2 sm:shrink-0">
        {thread.moderators.length > 0 && (
          <span>
            разбирает {thread.moderators.map((moderator) => moderator.username).join(', ')}
          </span>
        )}
        {thread.lastMessage && <span>{formatDateTime(thread.lastMessage.createdAt)}</span>}
        <span className="font-bold text-ink">{open ? 'Свернуть' : 'Открыть'}</span>
      </span>
    </button>
  )
}

/** Разговор с человеком: лента и ответ. Форма открыта только подключившемуся сотруднику. */
function Conversation({ thread }: { thread: SupportThread }) {
  const [text, setText] = useState('')
  const queryClient = useQueryClient()

  const { data: messages = [] } = useQuery({
    queryKey: supportKeys.messages(thread.id),
    queryFn: () => getSupportQueueMessages(thread.id),
    refetchInterval: (query) => (query.state.error ? false : 10_000),
  })

  const refresh = () => {
    queryClient.invalidateQueries({ queryKey: supportKeys.threads() })
    queryClient.invalidateQueries({ queryKey: supportKeys.messages(thread.id) })
  }

  const join = useMutation({ mutationFn: () => joinSupport(thread.id), onSuccess: refresh })
  const leave = useMutation({ mutationFn: () => leaveSupport(thread.id), onSuccess: refresh })
  const reply = useMutation({
    mutationFn: () => replySupport(thread.id, text.trim()),
    onSuccess: () => {
      setText('')
      refresh()
    },
  })

  // Подключён ли я — считаем по списку модераторов треда: сессия своей роли в нём не знает,
  // а бэкенд вернёт 403 на ответ от неподключённого.
  const joined = thread.moderators.length > 0

  return (
    <section className="flex flex-col gap-3 border-t border-line pt-3.5">
      <header className="flex items-center justify-between gap-3">
        <h2 className="text-[19px] leading-6 font-bold">Разговор с {thread.user.username}</h2>

        {joined ? (
          <Button variant="ghost" disabled={leave.isPending} onClick={() => leave.mutate()}>
            {leave.isPending ? 'Отключаемся…' : 'Отключиться'}
          </Button>
        ) : (
          <Button disabled={join.isPending} onClick={() => join.mutate()}>
            {join.isPending ? 'Подключаемся…' : 'Взять разговор'}
          </Button>
        )}
      </header>

      <ActionError error={join.error ?? leave.error ?? reply.error} />

      <ul className="flex flex-col gap-2">
        {messages.map((message) => (
          <li
            key={message.id}
            className={
              message.senderType === 'USER'
                ? 'max-w-[70%] self-start rounded-bubble bg-bubble-their px-3.5 py-2.5'
                : message.senderType === 'SYSTEM'
                  ? 'max-w-[70%] self-start rounded-bubble bg-bubble-system px-3.5 py-2.5'
                  : 'max-w-[70%] self-end rounded-bubble bg-bubble-mine px-3.5 py-2.5'
            }
          >
            <p className="text-[15px] leading-5">{message.text}</p>
            <p className="mt-0.5 text-[12px] leading-4 text-ink-3">
              {message.sender?.username ?? (message.senderType === 'SYSTEM' ? 'Сервис' : '')}{' '}
              {formatDateTime(message.createdAt)}
            </p>
          </li>
        ))}
      </ul>

      {/* Пока разговор не взят, форма закрыта: бэкенд всё равно ответит 403, а объяснить
          причину лучше здесь, чем сообщением об ошибке после набранного текста. */}
      {joined ? (
        <form
          className="flex gap-2"
          onSubmit={(event) => {
            event.preventDefault()
            if (text.trim()) reply.mutate()
          }}
        >
          <Input
            value={text}
            onChange={(event) => setText(event.target.value)}
            placeholder="Ответ человеку"
            aria-label="Ответ человеку"
          />
          <Button type="submit" disabled={reply.isPending || !text.trim()}>
            {reply.isPending ? 'Отправляем…' : 'Ответить'}
          </Button>
        </form>
      ) : (
        <p className="text-[13px] leading-4 text-ink-2">
          Возьмите разговор, чтобы ответить: так человек видит, кто именно ему пишет, а второй
          сотрудник — что вопрос уже разбирают.
        </p>
      )}
    </section>
  )
}
