import { IconImage } from '@/shared/ui'
import type { Item } from '../model/types'

/**
 * Вещь, которую человек отдаёт, — над формой желания. Раньше это была строчка
 * «Отдаёте: Монитор», но обмен начинается именно с расставания: карточка с фотографией
 * держит перед глазами, о чём вообще идёт речь, пока человек описывает, чего хочет.
 */
export function GivenItemCard({ item }: { item: Pick<Item, 'title' | 'category' | 'photoUrl'> }) {
  return (
    <div className="flex items-center gap-3 rounded-card border border-line p-3">
      {item.photoUrl ? (
        <img src={item.photoUrl} alt="" className="size-14 shrink-0 rounded-xl object-cover" />
      ) : (
        <div className="flex size-14 shrink-0 items-center justify-center rounded-xl bg-line-2 text-ink-3">
          <IconImage size={22} />
        </div>
      )}

      <div className="min-w-0">
        <b className="block truncate text-[15px] font-bold">{item.title}</b>
        <span className="block truncate text-[12.5px] text-ink-2">
          {[item.category, 'отдаёте в обмен'].filter(Boolean).join(' · ')}
        </span>
      </div>
    </div>
  )
}
