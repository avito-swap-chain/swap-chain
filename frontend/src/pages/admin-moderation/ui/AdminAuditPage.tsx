import { useQuery } from '@tanstack/react-query'
import { ACTION_LABEL, getAudit, moderationKeys } from '@/entities/moderation'
import { Card, EmptyState, Notice, Screen } from '@/shared/ui'

const formatDate = (iso: string): string =>
  new Date(iso).toLocaleString('ru-RU', {
    day: 'numeric',
    month: 'long',
    hour: '2-digit',
    minute: '2-digit',
  })

/**
 * Журнал административных действий. Дописываемый и неизменяемый: он существует ровно
 * затем, чтобы решение модератора можно было потом предъявить — в том числе ему самому.
 *
 * Содержимого сообщений и комментариев к решениям тут нет намеренно, так отдаёт бэкенд:
 * журнал отвечает на «кто и что сделал», а не пересказывает переписку.
 */
export function AdminAuditPage() {
  const { data, isPending, isError } = useQuery({
    queryKey: moderationKeys.audit(),
    queryFn: getAudit,
  })

  return (
    <Screen width="wide">
      <div className="flex flex-col gap-3.5 p-4">
        <h1 className="text-[22px] leading-7 font-bold lg:text-[32px] lg:leading-10">Журнал</h1>

        {isPending && <Notice>Загрузка…</Notice>}
        {isError && (
          <Notice tone="error">
            Не удалось загрузить журнал. Раздел открыт только модераторам.
          </Notice>
        )}

        {data && data.length === 0 && (
          <EmptyState
            title="Записей нет"
            description="Здесь появятся действия модераторов и события, которые их вызвали."
          />
        )}

        {data && data.length > 0 && (
          <div className="flex flex-col gap-2">
            {data.map((entry) => (
              <Card key={entry.id} padded className="flex flex-col gap-0.5">
                <p className="text-[14px] leading-5">
                  <b className="font-bold">{entry.admin.username}</b> — {ACTION_LABEL[entry.action]}
                </p>
                <p className="text-[13px] leading-4 text-ink-2">
                  {entry.targetType} №{entry.targetId} · {formatDate(entry.createdAt)}
                </p>
              </Card>
            ))}
          </div>
        )}
      </div>
    </Screen>
  )
}
