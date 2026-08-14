import type { ReactNode } from 'react'
import { cx } from '../lib'

/**
 * Красный счётчик непрочитанного — один на весь интерфейс. У Авито он везде одинаковый:
 * круг заливкой `accent-red`, белое число кеглем 11. У нас он был написан заново в пяти
 * местах — в шапке, меню кабинета, нижней панели, мессенджере и профиле, — и все пять
 * разошлись по кеглю, а профиль ещё и по цвету.
 *
 * Позиционирование остаётся на месте вызова: у иконки счётчик садится на угол абсолютом,
 * в строке меню стоит в потоке.
 */
export function Counter({
  children,
  title,
  className,
}: {
  children: ReactNode
  /** Что именно посчитано: число без подписи экранный читатель произносит впустую. */
  title?: string
  className?: string
}) {
  return (
    <span
      title={title}
      className={cx(
        'inline-grid min-w-4 place-items-center rounded-full bg-accent-red px-1 text-[11px] leading-4 font-bold text-white',
        className,
      )}
    >
      {children}
    </span>
  )
}
