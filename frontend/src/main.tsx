import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { RouterProvider } from 'react-router-dom'
import './index.css'
import { Providers } from './app/providers'
import { router } from './app/router'
import { ErrorBoundary } from './shared/ui'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    {/* Снаружи провайдеров: если упадут они сами или роутер целиком, показать всё равно
        есть что. Ошибки внутри экранов ловит `errorElement` роутера — там остаётся шапка. */}
    <ErrorBoundary>
      <Providers>
        <RouterProvider router={router} />
      </Providers>
    </ErrorBoundary>
  </StrictMode>,
)
