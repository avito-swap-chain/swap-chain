import { Avatar } from '@/shared/ui'
import type { ChainParticipant } from '../model/types'

/**
 * Лицо участника цепочки. Размер, фон и обводку задаёт место, где аватар стоит: в кольце он
 * крупный с обводкой, в списке — мелкий. Само «фото или инициал» живёт в `shared/ui/Avatar`:
 * так же выглядят аккаунт в шапке и профиль.
 */
export function ParticipantAvatar({
  participant,
  className,
}: {
  participant: ChainParticipant
  className?: string
}) {
  return <Avatar name={participant.name} src={participant.avatarUrl} className={className} />
}
