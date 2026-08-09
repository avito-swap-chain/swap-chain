import type { ReactNode } from 'react'
import { asset, cx } from '../lib'

/**
 * Пустой экран Авито устроен как разворот: слева заголовок, объяснение и одно действие,
 * справа — крупная иллюстрация. Пустота там не извиняется («ничего не найдено»), а говорит,
 * что здесь появится и что для этого сделать. На узких окнах разворот схлопывается в колонку.
 */
/**
 * Иллюстрации взяты у самого Авито (`public/illustrations`, источники — в README):
 * раздел обмена подаётся как часть их кабинета, и своя графика рядом с их же экранами
 * читалась бы как чужая вставка. Где у Авито картинки нет — например, в пустой выдаче
 * поиска, — не ставим её и мы.
 */
export type EmptyIllustration = 'items' | 'deals'

const ILLUSTRATIONS: Record<EmptyIllustration, { src: string; width: number; height: number }> = {
  items: { src: '/illustrations/empty-items.png', width: 140, height: 140 },
  deals: { src: '/illustrations/empty-deals.svg', width: 306, height: 288 },
}

export function EmptyState({
  title,
  description,
  action,
  illustration,
  className,
}: {
  title: string
  description: ReactNode
  action?: ReactNode
  illustration?: EmptyIllustration
  className?: string
}) {
  const picture = illustration ? ILLUSTRATIONS[illustration] : undefined

  return (
    <div
      className={cx(
        'flex flex-col items-center gap-8 py-10 text-center sm:flex-row sm:justify-between sm:text-left',
        className,
      )}
    >
      <div className="flex flex-col items-center gap-3 sm:items-start">
        <h3 className="text-[26px] leading-8 font-bold">{title}</h3>
        <p className="max-w-sm text-[15px] leading-5 text-ink-2">{description}</p>
        {action && <div className="mt-1">{action}</div>}
      </div>

      {picture && (
        <img
          src={asset(picture.src)}
          alt=""
          width={picture.width}
          height={picture.height}
          className="shrink-0 max-sm:order-first"
        />
      )}
    </div>
  )
}
