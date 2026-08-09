import { createBrowserRouter } from 'react-router-dom'
import { ChainPage } from '@/pages/chain'
import { CreateOfferPage } from '@/pages/create-offer'
import { EnableBarterPage } from '@/pages/enable-barter'
import { ExchangePage } from '@/pages/exchange'
import { ItemsPage } from '@/pages/items'
import { MessagesPage } from '@/pages/messages'
import { NotificationsPage } from '@/pages/notifications'
import { ThreadPage } from '@/pages/thread'
import { AvitoShell } from '@/widgets/avito-shell'

export const router = createBrowserRouter(
  [
    {
      // Layout-роут: все экраны раздела живут внутри оболочки Авито.
      element: <AvitoShell />,
      children: [
        // Верхний уровень — разделы меню кабинета.
        { path: '/', element: <ItemsPage /> },
        { path: '/exchange', element: <ExchangePage /> },
        { path: '/messages', element: <MessagesPage /> },
        { path: '/notifications', element: <NotificationsPage /> },
        // Вложенные экраны: сюда ведут из разделов, путь назад показывают крошки.
        { path: '/items/new', element: <CreateOfferPage /> },
        { path: '/items/:id/barter', element: <EnableBarterPage /> },
        { path: '/exchange/:id', element: <ChainPage /> },
        { path: '/messages/:itemId', element: <ThreadPage /> },
      ],
    },
  ],
  // На GitHub Pages сайт живёт в подпапке репозитория — роутер должен знать префикс,
  // иначе все ссылки уедут в корень домена. Значение подставляет Vite из `base`.
  { basename: import.meta.env.BASE_URL },
)
