import { asset, cx } from '../lib'

interface AvatarProps {
  /** Имя нужно и без фотографии: из него берётся инициал. */
  name: string
  src?: string
  /** Размер и обводку задаёт место: в кольце цепочки аватар крупный, в списке мелкий. */
  className?: string
}

/**
 * Лицо человека: фотография, а если её нет — инициал, как в аккаунтах Авито.
 * Выдумывать лицо тому, кто его не загрузил, нельзя — поэтому не заглушка-силуэт,
 * а первая буква имени.
 */
export function Avatar({ name, src, className }: AvatarProps) {
  const shape = cx('shrink-0 rounded-full', className)

  if (!src) {
    return (
      <span aria-hidden className={cx('grid place-items-center bg-line text-ink-2', shape)}>
        {name.slice(0, 1).toUpperCase()}
      </span>
    )
  }

  return <img src={asset(src)} alt="" loading="lazy" className={cx('object-cover', shape)} />
}
