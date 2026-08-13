import { useId, type ReactNode } from 'react'
import { IconPencil, Status } from '@/shared/ui'

interface ReviewBlockProps {
  label: string
  /** Что стоит в поле сейчас. Пусто — вместо значения показываем, чего не хватает. */
  value?: string
  missing: string
  /** Обязательное поле подсвечивается как ошибка, необязательное — как подсказка. */
  required?: boolean
  open: boolean
  onToggle: () => void
  /** Само поле — показывается, когда блок раскрыт. */
  children: ReactNode
}

/**
 * Строка сводки: что записано и карандаш, чтобы это поправить. Поле раскрывается на месте,
 * а не отдельным экраном — так правка видна рядом с остальными полями, и человек не теряет
 * из виду, что объявление уже почти готово.
 */
export function ReviewBlock({
  label,
  value,
  missing,
  required,
  open,
  onToggle,
  children,
}: ReviewBlockProps) {
  const id = useId()

  return (
    <section className="flex flex-col gap-2 border-b border-line-2 pb-4 last:border-b-0 last:pb-0">
      <div className="flex items-center justify-between gap-2">
        <h3 className="text-[15px] font-bold">{label}</h3>

        <button
          type="button"
          onClick={onToggle}
          aria-expanded={open}
          aria-controls={id}
          aria-label={open ? `Свернуть: ${label}` : `Изменить: ${label}`}
          className="cursor-pointer rounded-sm p-0.5 text-ink-2 outline-offset-4 hover:text-ink focus-visible:outline-2 focus-visible:outline-brand"
        >
          <IconPencil size={18} />
        </button>
      </div>

      <div id={id}>
        {open ? (
          children
        ) : value ? (
          <p className="text-[14px] leading-5 whitespace-pre-line text-ink-2">{value}</p>
        ) : (
          <Status tone={required ? 'stop' : 'muted'}>{missing}</Status>
        )}
      </div>
    </section>
  )
}
