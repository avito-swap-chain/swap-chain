import type { ReactNode } from 'react'
import { cx } from '@/shared/lib'
import { Card, IconImage } from '@/shared/ui'
import { CONDITION_LABEL } from '../model/dictionaries'
import type { Item } from '../model/types'
import { ItemStatusLabel } from './ItemStatusLabel'

interface ItemCardProps {
  item: Item
  /** Слот действия: сущность не знает про фичи, поэтому кнопку передаёт виджет. */
  action?: ReactNode
}

export function ItemCard({ item, action }: ItemCardProps) {
  return (
    // Кнопка до 400px переезжает под название: рядом с ней на заголовок остаётся полстроки,
    // и «Монитор LG 27" IPS» обрезается до «Монитор L…». Статус-подпись узкая — она
    // остаётся в строке всегда, отдельная строка ради неё вдвое растила бы карточку.
    <Card
      className={cx(
        'grid grid-cols-[auto_1fr] items-center gap-x-3 p-3',
        action ? 'gap-y-2.5 min-[400px]:flex' : 'grid-cols-[auto_1fr_auto]',
      )}
    >
      {item.photoUrl ? (
        <img
          src={item.photoUrl}
          alt=""
          className="size-13 shrink-0 rounded-xl object-cover"
          loading="lazy"
        />
      ) : (
        <div className="grid size-13 shrink-0 place-items-center rounded-xl bg-line-2 text-ink-3">
          <IconImage size={24} />
        </div>
      )}

      <div className="min-w-0 flex-1">
        <b className="block truncate text-[14.5px] font-bold">{item.title}</b>
        {/* Подпись собирается из того, что известно: по данным бэкенда категории и состояния
            нет, и строка «·» без частей выглядела бы поломкой. */}
        {(item.category || item.condition) && (
          <span className="block truncate text-[12.5px] text-ink-2">
            {[item.category, item.condition && CONDITION_LABEL[item.condition]]
              .filter(Boolean)
              .join(' · ')}
          </span>
        )}
      </div>

      <div
        className={cx(
          'flex justify-end',
          action ? 'col-span-2 min-[400px]:col-span-1 min-[400px]:shrink-0' : undefined,
        )}
      >
        {action ?? <ItemStatusLabel status={item.status} />}
      </div>
    </Card>
  )
}
