import { cx } from '@/shared/lib'
import type { Message } from '../model/types'
import { MessageBubble } from './MessageBubble'

/** Лента реплик. Прокрутка и высота — забота экрана, лента только раскладывает пузыри. */
export function MessageList({ messages, className }: { messages: Message[]; className?: string }) {
  return (
    <div className={cx('flex flex-col gap-1.5', className)}>
      {messages.map((message) => (
        <MessageBubble key={message.id} message={message} />
      ))}
    </div>
  )
}
