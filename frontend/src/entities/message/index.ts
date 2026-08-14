// Наружу отдаём только то, чем пользуются другие слои: остальное — внутренности слайса.
// Ленту реплик экраны читают через `useThreadMessages`, а не `getMessages`: там живут
// курсор, long-poll и склейка, и звать их руками негде.
export type {
  Message,
  MessageDraft,
  MessageReportReason,
  MessageRisk,
  ThreadRef,
} from './model/types'
export { MESSAGE_REPORT_REASONS, MESSAGE_REPORT_REASON_LABEL } from './model/types'
export {
  messageKeys,
  getThreads,
  sendMessage,
  markThreadRead,
  reportMessage,
  QUICK_QUESTIONS,
} from './api/messagesApi'
export { useThreadMessages } from './api/useThreadMessages'
export { mergeMessages } from './lib/mergeMessages'
export { isServiceThread, orderThreads, sameThread, threadPath } from './lib/thread'
export { MessageList } from './ui/MessageList'
export { ThreadCard } from './ui/ThreadCard'
