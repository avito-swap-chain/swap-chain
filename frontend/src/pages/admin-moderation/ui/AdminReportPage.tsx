import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate, useParams } from 'react-router-dom'
import type { ChatMessage, ReportDecision } from '@/shared/api/generated/model'
import {
  decide,
  getReport,
  moderationKeys,
  REASON_LABEL,
  ReportStatusLabel,
  takeReport,
} from '@/entities/moderation'
import { ActionError, Button, Card, Notice, Screen, ScreenHeader, Textarea } from '@/shared/ui'
import { cx } from '@/shared/lib'
import { getCurrentUser, sessionKeys } from '@/shared/model/session'

const formatTime = (iso: string): string =>
  new Date(iso).toLocaleString('ru-RU', {
    day: 'numeric',
    month: 'short',
    hour: '2-digit',
    minute: '2-digit',
  })

/**
 * Реплика в нити разговора. Та, на которую пожаловались, выделена: без этого разбирающий
 * ищет её глазами по идентификатору, а ошибиться здесь — значит решить не о том сообщении.
 */
function ContextMessage({ message, reported }: { message: ChatMessage; reported: boolean }) {
  return (
    <div
      className={cx(
        'rounded-card px-3 py-2 text-[14px] leading-5',
        reported ? 'bg-attention-bg ring-1 ring-attention' : 'bg-line-2',
      )}
    >
      <p className="text-[12.5px] leading-4 text-ink-2">
        {message.sender.username} · {formatTime(message.createdAt)}
      </p>
      <p className="mt-0.5 text-ink">{message.text}</p>
    </div>
  )
}

/**
 * Разбор жалобы. Решение терминальное и переписать его нельзя, поэтому оба исхода
 * равноправны на экране: «нарушения нет» — такой же результат разбора, как и подтверждение,
 * а не отмена действия.
 */
export function AdminReportPage() {
  const { reportId = '' } = useParams()
  const id = Number(reportId)
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const [comment, setComment] = useState('')

  const { data, isPending, isError } = useQuery({
    queryKey: moderationKeys.report(id),
    queryFn: () => getReport(id),
  })

  const refresh = () => {
    queryClient.invalidateQueries({ queryKey: moderationKeys.report(id) })
    queryClient.invalidateQueries({ queryKey: moderationKeys.all })
  }

  const take = useMutation({ mutationFn: () => takeReport(id), onSuccess: refresh })
  const resolve = useMutation({
    mutationFn: (decision: ReportDecision) => decide(id, decision, comment),
    onSuccess: refresh,
  })

  const { data: me } = useQuery({ queryKey: sessionKeys.current(), queryFn: getCurrentUser })

  const report = data?.report
  const open = report?.status === 'open'
  // Решение принимает только тот, кто взял жалобу: чужую бэкенд не даст закрыть, и форма
  // решения у неё была бы кнопкой, которая всегда отвечает отказом.
  const mine = Boolean(report?.assignee && me && String(report.assignee.id) === me.id)

  return (
    <Screen>
      <ScreenHeader title="Жалоба" onBack={() => navigate('/admin/moderation')} />

      <div className="flex flex-col gap-3.5 p-4">
        {isPending && <Notice>Загрузка…</Notice>}
        {isError && (
          <Notice tone="error">
            Не удалось загрузить жалобу. Раздел открыт только модераторам.
          </Notice>
        )}

        {data && report && (
          <>
            <Card padded className="flex flex-col gap-1">
              <div className="flex items-center justify-between gap-3">
                <p className="text-[15px] leading-5 font-bold">{REASON_LABEL[report.reason]}</p>
                <ReportStatusLabel status={report.status} />
              </div>
              <p className="text-[13px] leading-4 text-ink-2">
                {report.reporter.username} · {formatTime(report.createdAt)}
                {report.assignee && ` · разбирает ${report.assignee.username}`}
              </p>
              {report.comment && (
                <p className="mt-1 text-[13.5px] leading-5 text-ink">{report.comment}</p>
              )}
              {/* Решение уже принято — показываем его вместо кнопок: переписать нельзя. */}
              {report.decisionComment && (
                <p className="mt-2 border-t border-line-2 pt-2 text-[13.5px] leading-5 text-ink-2">
                  Решение: {report.decisionComment}
                </p>
              )}
            </Card>

            <section className="flex flex-col gap-1.5">
              <h2 className="text-[15px] font-bold">Разговор</h2>
              {data.context.map((message) => (
                <ContextMessage
                  key={message.id}
                  message={message}
                  reported={message.id === data.reportedMessage.id}
                />
              ))}
            </section>

            {/* Жалобу разбирает кто-то другой. Кнопку не показываем — 409 в ответ на неё
                объяснил бы то же самое, но после нажатия. */}
            {open && !mine && report.assignee && (
              <Notice>Жалобу разбирает {report.assignee.username}</Notice>
            )}

            {open && !mine && !report.assignee && (
              <div className="flex flex-col gap-1">
                <Button disabled={take.isPending} onClick={() => take.mutate()}>
                  {take.isPending ? 'Берём…' : 'Взять в разбор'}
                </Button>
                {/* Занятую жалобу бэкенд отдаёт 409 — её взял другой модератор. */}
                <ActionError error={take.error} conflict="Жалобу уже взял другой модератор" />
              </div>
            )}

            {open && mine && (
              <section className="flex flex-col gap-2.5 rounded-card border border-line p-3">
                <h2 className="text-[15px] font-bold">Решение</h2>
                {/* Комментарий обязателен: в журнал попадает он, а не сам факт решения,
                    и «резолвед» без причины не объяснит ничего ни через день, ни в споре. */}
                <Textarea
                  value={comment}
                  onChange={(event) => setComment(event.target.value)}
                  placeholder="Что решили и почему — это останется в журнале"
                  aria-label="Комментарий к решению"
                  rows={3}
                  maxLength={1000}
                />

                <div className="flex flex-wrap gap-2">
                  <Button
                    variant="danger"
                    disabled={!comment.trim() || resolve.isPending}
                    onClick={() => resolve.mutate('resolved')}
                  >
                    Нарушение есть
                  </Button>
                  <Button
                    variant="ghost"
                    disabled={!comment.trim() || resolve.isPending}
                    onClick={() => resolve.mutate('rejected')}
                  >
                    Нарушения нет
                  </Button>
                </div>

                <ActionError error={resolve.error} />
              </section>
            )}
          </>
        )}
      </div>
    </Screen>
  )
}
