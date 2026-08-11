import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import {
  mergeMessages,
  messageKeys,
  QUICK_QUESTIONS,
  sendMessage,
  type Message,
  type MessageDraft,
  type ThreadRef,
} from '@/entities/message'
import { ActionError, Button, Input } from '@/shared/ui'

/**
 * Отправка сообщения: поле ввода и заготовки вопросов о состоянии. Один компонент
 * на оба места — панель у предложения и экран переписки, — иначе разговор, начатый
 * в одном месте, вёлся бы по другим правилам в другом.
 *
 * Заготовки показываем, пока разговор не начат: дальше человек спрашивает сам.
 */
export function MessageComposer({ thread, empty }: { thread: ThreadRef; empty: boolean }) {
  const [draft, setDraft] = useState('')
  const queryClient = useQueryClient()

  const { mutate, isPending, error } = useMutation({
    mutationFn: (message: MessageDraft) => sendMessage(thread, message),
    // Повтор идёт с тем же ключом идемпотентности — он в переменных мутации, а не внутри
    // запроса. Ради этого повтор и включён: обрыв на ответе сервера иначе оставляет человека
    // гадать, дошло ли, а вторая попытка вернёт исходную реплику вместо дубля.
    retry: 1,
    onSuccess: (sent) => {
      // Своё сообщение показываем сразу, не дожидаясь опроса, — и тем же слиянием,
      // чтобы пришедшее следом из long-poll не задвоило его.
      queryClient.setQueryData<Message[]>(messageKeys.thread(thread), (known = []) =>
        mergeMessages(known, [sent]),
      )
      queryClient.invalidateQueries({ queryKey: messageKeys.list() })
    },
    // Не ушло — возвращаем текст в поле. Поле очищается сразу при отправке, и без этого
    // сообщение просто исчезало бы: человек считает, что отправил, а реплики нет.
    onError: (_, message) => setDraft(message.text),
  })

  const send = (text: string) => {
    const trimmed = text.trim()
    if (!trimmed || isPending) return
    mutate({ text: trimmed, clientMessageId: crypto.randomUUID() })
    setDraft('')
  }

  return (
    <div className="flex flex-col gap-2.5">
      {empty && (
        <ul className="flex flex-wrap gap-1.5">
          {QUICK_QUESTIONS.map((question) => (
            <li key={question}>
              <button
                type="button"
                disabled={isPending}
                onClick={() => send(question)}
                className="rounded-chip bg-line-2 px-2.5 py-1.5 text-[13px] font-semibold text-ink outline-offset-2 hover:bg-line focus-visible:outline-2 focus-visible:outline-brand disabled:opacity-50"
              >
                {question}
              </button>
            </li>
          ))}
        </ul>
      )}

      <form
        className="flex gap-2"
        onSubmit={(event) => {
          event.preventDefault()
          send(draft)
        }}
      >
        <Input
          value={draft}
          onChange={(event) => setDraft(event.target.value)}
          placeholder="Сообщение"
          aria-label="Сообщение"
        />
        <Button type="submit" disabled={isPending || !draft.trim()}>
          Отправить
        </Button>
      </form>

      <ActionError error={error} conflict="Переписка недоступна — цепочка уже завершилась" />
    </div>
  )
}
