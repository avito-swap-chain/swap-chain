import { useState, type SubmitEvent } from 'react'
import { useMutation } from '@tanstack/react-query'
import { REPORT_REASONS, REPORT_REASON_LABEL, reportUser, type ReportReason } from '@/entities/user'
import { ActionError, Button, IconCheck, IconFlag, TileGroup, TileRow, Textarea } from '@/shared/ui'

/**
 * Пожаловаться на человека. Причина выбирается из короткого списка, текст необязателен:
 * причину читает счётчик, текст — человек.
 *
 * С контракта 0.10.0 жалоба уходит на бэкенд и попадает в очередь модерации — ту же, куда
 * приходят жалобы на реплики. Поэтому подтверждение теперь может обещать разбор: за ним
 * действительно кто-то стоит.
 */
export function ReportUser({ userId, chainId }: { userId: string; chainId?: string }) {
  const [open, setOpen] = useState(false)
  const [reason, setReason] = useState<ReportReason>(REPORT_REASONS[0])
  const [text, setText] = useState('')

  const { mutate, isPending, isSuccess, error } = useMutation({
    mutationFn: () => reportUser({ targetUserId: userId, chainId, reason, text: text.trim() }),
  })

  const submit = (event: SubmitEvent<HTMLFormElement>) => {
    event.preventDefault()
    mutate()
  }

  if (isSuccess) {
    return (
      <p className="flex items-center gap-2 rounded-card bg-ok-bg px-4 py-3 text-[13.5px] leading-5">
        <IconCheck size={17} className="shrink-0 text-ok" />
        Жалоба отправлена — её разберёт модератор.
      </p>
    )
  }

  return (
    <div className="flex flex-col gap-2">
      <TileGroup>
        <TileRow icon={<IconFlag size={18} />} onClick={() => setOpen((shown) => !shown)}>
          Пожаловаться
        </TileRow>
      </TileGroup>

      {open && (
        <form onSubmit={submit} className="flex flex-col gap-3 rounded-card border border-line p-3">
          <fieldset className="flex flex-col gap-2">
            <legend className="mb-1.5 font-sans text-xs font-semibold text-ink-2">
              Что случилось
            </legend>

            {REPORT_REASONS.map((option) => (
              <label key={option} className="flex cursor-pointer items-center gap-2.5">
                <input
                  type="radio"
                  name="reason"
                  value={option}
                  checked={reason === option}
                  onChange={() => setReason(option)}
                  className="size-4 accent-brand"
                />
                <span className="text-[13.5px] leading-5">{REPORT_REASON_LABEL[option]}</span>
              </label>
            ))}
          </fieldset>

          <Textarea
            value={text}
            onChange={(event) => setText(event.target.value)}
            placeholder={
              reason === 'other'
                ? 'Что произошло — своими словами'
                : 'Что произошло — своими словами. Необязательно'
            }
            aria-label="Подробности жалобы"
            rows={3}
            /* Потолок контракта. Был 2000 — форма принимала текст, который бэкенд отвергал. */
            maxLength={1000}
            required={reason === 'other'}
          />

          <div className="flex gap-2">
            <Button type="submit" disabled={isPending}>
              {isPending ? 'Отправляем…' : 'Отправить жалобу'}
            </Button>
            <Button variant="ghost" onClick={() => setOpen(false)}>
              Отмена
            </Button>
          </div>

          <ActionError error={error} />
        </form>
      )}
    </div>
  )
}
