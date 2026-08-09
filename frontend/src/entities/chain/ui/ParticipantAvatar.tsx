import { cx } from '@/shared/lib'
import type { ChainParticipant } from '../model/types'

/**
 * Лицо участника: фото профиля, а если его нет — инициал, как в аккаунтах Авито.
 * Размер, фон и обводку задаёт место, где аватар стоит: в кольце он крупный
 * с обводкой, в списке — мелкий.
 */
export function ParticipantAvatar({
  participant,
  className,
}: {
  participant: ChainParticipant
  className?: string
}) {
  const shape = cx('shrink-0 rounded-full', className)

  if (!participant.avatarUrl) {
    return (
      <span className={cx('grid place-items-center', shape)} aria-hidden="true">
        {participant.name.slice(0, 1)}
      </span>
    )
  }

  return (
    <img src={participant.avatarUrl} alt="" loading="lazy" className={cx('object-cover', shape)} />
  )
}
