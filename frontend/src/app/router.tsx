import { createBrowserRouter } from 'react-router-dom'
import { ChainPage } from '@/pages/chain'
import { CreateOfferPage } from '@/pages/create-offer'
import { EnableBarterPage } from '@/pages/enable-barter'
import { ExchangePage } from '@/pages/exchange'
import { ItemsPage } from '@/pages/items'
import { AvitoShell } from '@/widgets/avito-shell'

export const router = createBrowserRouter([
  {
    // Layout-роут: все экраны раздела живут внутри оболочки Авито.
    element: <AvitoShell />,
    children: [
      // Верхний уровень — ровно два раздела, они же пункты меню кабинета.
      { path: '/', element: <ItemsPage /> },
      { path: '/exchange', element: <ExchangePage /> },
      // Вложенные экраны: сюда ведут из разделов, путь назад показывают крошки.
      { path: '/items/new', element: <CreateOfferPage /> },
      { path: '/items/:id/barter', element: <EnableBarterPage /> },
      { path: '/exchange/:id', element: <ChainPage /> },
    ],
  },
])
