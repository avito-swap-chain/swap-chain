import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import type { ListAdminReportsParams, MessageReport } from '@/shared/api/generated/model'
import { getReports, moderationKeys, REASON_LABEL, ReportStatusLabel } from '@/entities/moderation'
import { Button, Card, EmptyState, Notice, Screen } from '@/shared/ui'

/**
 * Что показывать в очереди. «Свободные» — жалобы, которые ещё никто не взял: с них разбор
 * и начинают, поэтому фильтр открыт на них, а не на всём подряд.
 */
const VIEWS: { key: string; label: string; params: ListAdminReportsParams }[] = [
  { key: 'unassigned', label: 'Свободные', params: { status: 'open', unassigned: true } },
  { key: 'open', label: 'В очереди', params: { status: 'open' } },
  { key: 'all', label: 'Все', params: {} },
]

const formatDate = (iso: string): string =>
  new Date(iso).toLocaleString('ru-RU', {
    day: 'numeric',
    month: 'long',
    hour: '2-digit',
    minute: '2-digit',
  })

function ReportRow({ report }: { report: MessageReport }) {
  return (
    <Card padded className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <div className="min-w-0">
        <p className="text-[15px] leading-5 font-bold">{REASON_LABEL[report.reason]}</p>
        <p className="mt-0.5 text-[13px] leading-4 text-ink-2">
          {report.reporter.username} · {formatDate(report.createdAt)}
          {/* Кто взял жалобу — единственное, что отличает две одинаковые строки в очереди. */}
          {report.assignee && ` · разбирает ${report.assignee.username}`}
        </p>
        {report.comment && (
          <p className="mt-1 line-clamp-2 text-[13.5px] leading-5 text-ink-2">{report.comment}</p>
        )}
      </div>

      <div className="flex items-center gap-4 sm:shrink-0">
        <ReportStatusLabel status={report.status} />
        <Link to={`/admin/moderation/${report.id}`}>
          <Button variant="ghost">Открыть</Button>
        </Link>
      </div>
    </Card>
  )
}

/**
 * Очередь жалоб. Жалоба всегда о конкретной реплике, поэтому строка сама по себе решения
 * не даёт — разбирают на экране жалобы, где видно и сообщение, и разговор вокруг него.
 */
export function AdminModerationPage() {
  const [view, setView] = useState(VIEWS[0])

  const { data, isPending, isError } = useQuery({
    queryKey: moderationKeys.reports(view.params),
    queryFn: () => getReports(view.params),
  })

  return (
    <Screen width="wide">
      <div className="flex flex-col gap-3.5 p-4">
        <h1 className="text-[22px] leading-7 font-bold lg:text-[32px] lg:leading-10">Жалобы</h1>

        <div className="flex gap-2">
          {VIEWS.map((option) => (
            <Button
              key={option.key}
              variant={option.key === view.key ? 'primary' : 'ghost'}
              onClick={() => setView(option)}
            >
              {option.label}
            </Button>
          ))}
        </div>

        {isPending && <Notice>Загрузка…</Notice>}
        {/* Сюда же приходит 403: роль проверяет бэкенд, своей проверки на фронте нет —
            вторая, необязательная, только разошлась бы с настоящей. */}
        {isError && (
          <Notice tone="error">
            Не удалось загрузить жалобы. Раздел открыт только модераторам.
          </Notice>
        )}

        {data && data.length === 0 && (
          <EmptyState
            title="Жалоб нет"
            description="Здесь появятся жалобы на сообщения — по одной на каждую реплику, о которой сообщили."
          />
        )}

        {data && data.length > 0 && (
          <div className="flex flex-col gap-2">
            {data.map((report) => (
              <ReportRow key={report.id} report={report} />
            ))}
          </div>
        )}
      </div>
    </Screen>
  )
}
