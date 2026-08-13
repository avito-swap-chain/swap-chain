import type { ReactNode } from 'react'
import { cx } from '@/shared/lib'
import type { Message } from '../model/types'
import { MessageBubble } from './MessageBubble'

interface MessageListProps {
  messages: Message[]
  className?: string
  /** Что можно сделать с репликой. Решает экран: лента про раскладку, а не про действия. */
  action?: (message: Message) => ReactNode
}

/** Лента реплик. Прокрутка и высота — забота экрана, лента только раскладывает пузыри. */
export function MessageList({ messages, className, action }: MessageListProps) {
  return (
    <div className={cx('flex flex-col gap-1.5', className)}>
      {messages.map((message) => (
        <MessageBubble key={message.id} message={message} action={action?.(message)} />
      ))}
    </div>
  )
}
