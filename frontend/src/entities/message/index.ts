// Наружу отдаём только то, чем пользуются другие слои: остальное — внутренности слайса.
// Ленту реплик экраны читают через `useThreadMessages`, а не `getMessages`: там живут
// курсор, long-poll и склейка, и звать их руками негде.
export type { Message, MessageDraft, ThreadRef } from './model/types'
export {
  messageKeys,
  getThreads,
  sendMessage,
  markThreadRead,
  QUICK_QUESTIONS,
} from './api/messagesApi'
export { useThreadMessages } from './api/useThreadMessages'
export { mergeMessages } from './lib/mergeMessages'
export { isServiceThread, sameThread, threadPath } from './lib/thread'
export { MessageList } from './ui/MessageList'
export { ThreadCard } from './ui/ThreadCard'
