import { useQuery } from '@tanstack/react-query'
import { ACTION_LABEL, getAudit, moderationKeys } from '@/entities/moderation'
import { formatDateTime } from '@/shared/lib'
import { EmptyState, Notice, Screen } from '@/shared/ui'

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
          <div className="flex flex-col">
            {/* Строками через разделитель: журнал читают сверху вниз, а рамка вокруг
                каждой записи превращала ленту в стопку карточек. */}
            {data.map((entry) => (
              <div
                key={entry.id}
                className="flex flex-col gap-0.5 border-b border-line py-3 last:border-0"
              >
                <p className="text-[14px] leading-5">
                  <b className="font-bold">{entry.admin.username}</b> — {ACTION_LABEL[entry.action]}
                </p>
                <p className="text-[13px] leading-4 text-ink-2">
                  {entry.targetType} №{entry.targetId} · {formatDateTime(entry.createdAt)}
                </p>
              </div>
            ))}
          </div>
        )}
      </div>
    </Screen>
  )
}
