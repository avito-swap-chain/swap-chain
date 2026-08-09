import { useQuery } from '@tanstack/react-query'
import { getThreads, messageKeys, ThreadCard } from '@/entities/message'
import { Notice } from '@/shared/ui'

/**
 * Список переписок. Разговоры заводятся у предложения обмена — сюда они попадают сами,
 * поэтому «пустого» состояния с призывом написать первым здесь нет: писать наугад,
 * не видя варианта обмена, всё равно некому.
 */
export function ThreadsList() {
  const { data, isPending, isError } = useQuery({
    queryKey: messageKeys.list(),
    queryFn: getThreads,
  })

  if (isPending) return <Notice>Загрузка…</Notice>
  if (isError) return <Notice tone="error">Не удалось загрузить сообщения</Notice>

  return (
    <div className="flex flex-col">
      {data.map((thread) => (
        <ThreadCard key={thread.itemId} thread={thread} />
      ))}
    </div>
  )
}
