import { cx } from '@/shared/lib'
import type { Message } from '../model/types'

/* Геометрия и цвета сняты с мессенджера Авито: пузырь — 20px со срезанным до 4px углом
 * со стороны автора, отступы 8/12/12 (снизу больше — под строку времени), кегль 15/20.
 * Свой пузырь — palette-blue50, чужой — bg-default, служебный — palette-green50. */
const tones: Record<Message['author'], string> = {
  me: 'self-end bg-bubble-mine rounded-br-[4px]',
  them: 'self-start bg-bubble-their rounded-bl-[4px]',
  system: 'self-start bg-bubble-system rounded-bl-[4px]',
}

/** Часы и минуты отправки — как в ленте Авито. */
const formatTime = (iso: string): string =>
  new Date(iso).toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' })

export function MessageBubble({ message }: { message: Message }) {
  return (
    <div
      className={cx(
        'max-w-[80%] rounded-bubble px-3 pt-2 pb-3 text-[15px] leading-5 text-ink',
        tones[message.author],
      )}
    >
      {message.text}
      <span className="mt-0.5 block text-right text-[13px] leading-4 text-ink-2">
        {formatTime(message.createdAt)}
      </span>
    </div>
  )
}
