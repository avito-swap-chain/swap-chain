import { useState, type SubmitEvent } from 'react'
import { useMutation } from '@tanstack/react-query'
import {
  MESSAGE_REPORT_REASONS,
  MESSAGE_REPORT_REASON_LABEL,
  reportMessage,
  type MessageReportReason,
} from '@/entities/message'
import { ActionError, Button, Textarea } from '@/shared/ui'

/**
 * Пожаловаться на реплику собеседника. Жалоба привязана к конкретному сообщению, а не
 * к человеку: модератор читает то же, что прочитал пострадавший, и разбирать ему есть что.
 *
 * Кнопка живёт под самим пузырём и появляется по наведению или фокусу: жалоба нужна редко,
 * а подпись под каждой репликой превратила бы разговор в форму.
 */
export function ReportMessage({ messageId }: { messageId: string }) {
  const [open, setOpen] = useState(false)
  const [reason, setReason] = useState<MessageReportReason>(MESSAGE_REPORT_REASONS[0])
  const [comment, setComment] = useState('')

  const { mutate, isPending, isSuccess, error } = useMutation({
    mutationFn: () => reportMessage(messageId, reason, comment),
  })

  const submit = (event: SubmitEvent<HTMLFormElement>) => {
    event.preventDefault()
    mutate()
  }

  // Подтверждение обещает только приём: очередь модерации есть, но сроков разбора у неё
  // нет, и называть их значило бы придумывать.
  if (isSuccess) {
    return <p className="text-[12.5px] leading-4 text-ok">Жалоба отправлена</p>
  }

  if (!open) {
    return (
      <button
        type="button"
        onClick={() => setOpen(true)}
        className="w-fit cursor-pointer rounded-sm text-[12.5px] font-semibold text-ink-3 opacity-0 outline-offset-2 group-focus-within:opacity-100 group-hover:opacity-100 hover:text-ink-2 focus-visible:opacity-100 focus-visible:outline-2 focus-visible:outline-brand"
      >
        Пожаловаться
      </button>
    )
  }

  // Ширина формы задана явно и без `max-w-full`: колонка пузыря тянется по содержимому,
  // и ограничение в 100% замыкалось само на себя — форма съезжала обратно к ширине реплики,
  // в которой переносится даже подпись поля. 300px влезают и в самое узкое окно.
  return (
    <form
      onSubmit={submit}
      className="flex w-[300px] flex-col gap-2.5 rounded-card border border-line p-3"
    >
      <fieldset className="flex flex-col gap-2">
        <legend className="mb-1.5 font-sans text-xs font-semibold text-ink-2">Что не так</legend>

        {MESSAGE_REPORT_REASONS.map((option) => (
          <label key={option} className="flex cursor-pointer items-center gap-2.5">
            <input
              type="radio"
              name={`report-${messageId}`}
              value={option}
              checked={reason === option}
              onChange={() => setReason(option)}
              className="size-4 accent-brand"
            />
            <span className="text-[13.5px] leading-5">{MESSAGE_REPORT_REASON_LABEL[option]}</span>
          </label>
        ))}
      </fieldset>

      {/* У «другого» текст — единственное, из чего понятно, в чём дело: бэкенд без него
          жалобу не принимает, и спрашиваем мы ровно там, где это становится обязательным. */}
      <Textarea
        value={comment}
        onChange={(event) => setComment(event.target.value)}
        placeholder={
          reason === 'other' ? 'Что произошло — своими словами' : 'Подробности. Необязательно'
        }
        aria-label="Подробности жалобы"
        rows={2}
        maxLength={1000}
      />

      <div className="flex gap-2">
        <Button type="submit" disabled={isPending}>
          {isPending ? 'Отправляем…' : 'Отправить'}
        </Button>
        <Button variant="ghost" onClick={() => setOpen(false)}>
          Отмена
        </Button>
      </div>

      <ActionError error={error} />
    </form>
  )
}
