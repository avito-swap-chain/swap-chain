import type { ReactNode } from 'react'
import { cx } from '@/shared/lib'
import type { Message, MessageRisk } from '../model/types'

/* Геометрия и цвета сняты с мессенджера Авито: пузырь — 20px со срезанным до 4px углом
 * со стороны автора, отступы 8/12/12 (снизу больше — под строку времени), кегль 15/20.
 * Свой пузырь — palette-blue50, чужой — bg-default, служебный — palette-green50. */
const tones: Record<Message['author'], string> = {
  me: 'bg-bubble-mine rounded-br-[4px]',
  them: 'bg-bubble-their rounded-bl-[4px]',
  system: 'bg-bubble-system rounded-bl-[4px]',
}

/* Сторона ленты переехала на обёртку: под пузырём теперь может стоять действие,
 * и прижиматься к краю должны они вместе, а не пузырь сам по себе. */
const sides: Record<Message['author'], string> = {
  me: 'self-end items-end',
  them: 'self-start items-start',
  system: 'self-start items-start',
}

/**
 * Чем опасна реплика — словами, а не кодом. Бэкенд размечает текст детерминированно
 * и ничего не блокирует: обмен строится на разговоре незнакомых людей, и запрещать им
 * писать нельзя. Но сказать вслух, что у сервиса никогда не спрашивают код из СМС,
 * надо ровно там, где этот код просят.
 */
const RISK_WARNING: Record<MessageRisk, string> = {
  CREDENTIALS: 'У вас просят пароль или логин. Сервису они не нужны — не отправляйте.',
  VERIFICATION_CODE:
    'У вас просят код из СМС. Его не спрашивает ни Авито, ни другой участник обмена.',
  PAYMENT_DETAILS:
    'Речь о деньгах или карте. В обмене нет ни цен, ни доплат — вещь меняется на вещь.',
  EXTERNAL_LINK: 'В сообщении внешняя ссылка. Проверьте, куда она ведёт, прежде чем открывать.',
  OFF_PLATFORM_CONTACT:
    'Вас зовут в другой мессенджер. Там переписку не увидит поддержка, и жалобу разобрать будет нечем.',
}

/** Часы и минуты отправки — как в ленте Авито. */
const formatTime = (iso: string): string =>
  new Date(iso).toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' })

/**
 * Реплика. `action` — то, что можно сделать с чужим сообщением; кто именно, решает экран:
 * сущность не знает про жалобы, а лента без действий должна оставаться просто лентой.
 */
export function MessageBubble({ message, action }: { message: Message; action?: ReactNode }) {
  return (
    <div className={cx('group flex max-w-[80%] flex-col gap-1', sides[message.author])}>
      <div
        className={cx(
          'rounded-bubble px-3 pt-2 pb-3 text-[15px] leading-5 text-ink',
          tones[message.author],
        )}
      >
        {message.text}
        <span className="mt-0.5 block text-right text-[13px] leading-4 text-ink-2">
          {formatTime(message.createdAt)}
        </span>
      </div>

      {/* Предупреждение под пузырём, а не вместо него: сообщение человек всё равно должен
          прочитать сам — мы только называем риск. */}
      {message.risk && (
        <p
          role="note"
          className="max-w-full rounded-chip bg-attention-bg px-2.5 py-1.5 text-[12.5px] leading-4 text-ink"
        >
          {RISK_WARNING[message.risk]}
        </p>
      )}

      {action}
    </div>
  )
}
