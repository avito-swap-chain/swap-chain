import { Screen } from '@/shared/ui'
import { ThreadsList } from '@/widgets/threads-list'

/**
 * Раздел «Сообщения» — тот же мессенджер, что и у Авито, только все разговоры здесь
 * про обмен: тема каждого — конкретная вещь из предложения.
 */
export function MessagesPage() {
  return (
    <Screen width="wide">
      <div className="flex flex-col gap-3.5 p-4">
        <h1 className="text-[22px] leading-7 font-bold lg:text-[32px] lg:leading-10">Сообщения</h1>
        <ThreadsList />
      </div>
    </Screen>
  )
}
