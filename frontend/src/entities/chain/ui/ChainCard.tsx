import { Card } from '@/shared/ui'
import { findMe, findNeighbours, needsMyAction } from '../lib/participants'
import type { Chain } from '../model/types'
import { DealStatusLabel } from './DealStatusLabel'

/** Что происходит с цепочкой — одной строкой, языком пользователя. */
function hintFor(chain: Chain): string {
  switch (chain.status) {
    case 'formed':
      if (needsMyAction(chain)) return 'ждём вашего решения'
      // Отказавшийся ничего не ждёт: цепочка живёт без него, пока ему ищут замену.
      return findMe(chain)?.status === 'declined'
        ? 'вы отказались от варианта'
        : 'ждём остальных участников'
    case 'active':
      return needsMyAction(chain) ? 'отметьте получение вещи' : 'ждём остальных участников'
    case 'completed':
      return 'обмен завершён'
    case 'dissolved':
      return 'цепочка распалась'
    case 'cancelled':
      return 'вещь ушла в другой обмен'
  }
}

/** Строка списка «Мои обмены»: что отдаю, что получаю и на каком этапе цепочка. */
export function ChainCard({ chain }: { chain: Chain }) {
  const me = findMe(chain)
  const neighbours = findNeighbours(chain)
  if (!me || !neighbours) return null

  return (
    <Card className="flex items-center gap-3 p-3">
      <div className="min-w-0 flex-1">
        <b className="block truncate text-[14.5px] font-bold">
          {me.givesItem.title} → {neighbours.giver.givesItem.title}
        </b>
        {/* Строка нейтральна, даже когда ход за пользователем: цвет в карточке несёт
            только статус справа — иначе оранжевого столько, что он перестаёт звать. */}
        <span className="block truncate text-[12.5px] text-ink-2">
          Участников: {chain.participants.length} · {hintFor(chain)}
        </span>
      </div>

      <DealStatusLabel chain={chain} />
    </Card>
  )
}
