import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { messageKeys, QUICK_QUESTIONS, sendMessage, type ThreadRef } from '@/entities/message'
import { Button, Input } from '@/shared/ui'

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

  const { mutate, isPending } = useMutation({
    mutationFn: (text: string) => sendMessage({ ...thread, text }),
    onSuccess: (updated) => {
      queryClient.setQueryData(messageKeys.thread(thread.itemId), updated)
      queryClient.invalidateQueries({ queryKey: messageKeys.list() })
    },
  })

  const send = (text: string) => {
    const trimmed = text.trim()
    if (!trimmed || isPending) return
    mutate(trimmed)
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
    </div>
  )
}
