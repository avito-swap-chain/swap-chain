import { Component, type ErrorInfo, type ReactNode } from 'react'
import { Button } from './Button'

interface ErrorBoundaryProps {
  children: ReactNode
}

interface ErrorBoundaryState {
  failed: boolean
}

/**
 * Последний рубеж: исключение при рендере не должно оставлять человека перед белым экраном.
 * Ловим классовым компонентом, потому что другого способа в React нет — хука для этого
 * не существует.
 *
 * Перезагрузка, а не «попробовать ещё раз» без перезагрузки: если рендер уже сломался
 * на текущих данных, повторная отрисовка того же состояния сломается снова.
 */
export class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  state: ErrorBoundaryState = { failed: false }

  static getDerivedStateFromError(): ErrorBoundaryState {
    return { failed: true }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    // Логи — единственное, что здесь можно сделать полезного: сборщика ошибок в MVP нет,
    // а тихо проглотить причину значит остаться без отладки на демо.
    console.error('Экран упал:', error, info.componentStack)
  }

  render() {
    if (!this.state.failed) return this.props.children

    return (
      <div className="flex min-h-svh items-center justify-center bg-page px-4">
        <div className="flex w-full max-w-[380px] flex-col gap-3.5 rounded-2xl bg-card p-6 text-center">
          <h1 className="text-[20px] leading-6 font-bold">Что-то сломалось</h1>
          <p className="text-[14px] leading-5 text-ink-2">
            Мы не смогли показать этот экран. Обновите страницу — обычно этого достаточно.
          </p>
          <Button fullWidth onClick={() => window.location.reload()}>
            Обновить страницу
          </Button>
        </div>
      </div>
    )
  }
}
