import { createBrowserRouter } from 'react-router-dom'
import { AdminDeliveriesPage } from '@/pages/admin-deliveries'
import { AdminAuditPage, AdminModerationPage, AdminReportPage } from '@/pages/admin-moderation'
import { BlockedPage } from '@/pages/blocked'
import { ChainPage } from '@/pages/chain'
import { CreateOfferPage } from '@/pages/create-offer'
import { EditItemPage } from '@/pages/edit-item'
import { EnableBarterPage } from '@/pages/enable-barter'
import { ExchangePage } from '@/pages/exchange'
import { ItemsPage } from '@/pages/items'
import { MessagesPage } from '@/pages/messages'
import { NotificationsPage } from '@/pages/notifications'
import { ProfilePage } from '@/pages/profile'
import { ReviewsPage } from '@/pages/reviews'
import { ThreadPage } from '@/pages/thread'
import { AvitoShell } from '@/widgets/avito-shell'
import { MiniMessenger } from '@/widgets/mini-messenger'
import { RouteError } from './RouteError'
import { SessionGate } from './SessionGate'

export const router = createBrowserRouter(
  [
    {
      // Layout-роут: все экраны раздела живут внутри оболочки Авито и за входом.
      element: (
        <SessionGate>
          <AvitoShell />
          {/* Плавающий мессенджер живёт поверх всех экранов кабинета, поэтому подключён
              здесь, а не внутри оболочки: два виджета одного слоя друг о друге не знают. */}
          <MiniMessenger />
        </SessionGate>
      ),
      // Исключение при рендере любого экрана: без этого роутер показал бы служебную
      // страницу с трассировкой — на демо это выглядит как падение сервиса.
      errorElement: <RouteError />,
      children: [
        // Верхний уровень — разделы меню кабинета.
        { path: '/', element: <ItemsPage /> },
        { path: '/exchange', element: <ExchangePage /> },
        { path: '/messages', element: <MessagesPage /> },
        // «Мои отзывы» — отдельный раздел меню, как у Авито: рейтинг в обмене это довод,
        // а не украшение профиля.
        { path: '/reviews', element: <ReviewsPage /> },
        // Чёрный список — вторая половина блокировки: заблокировать можно было из профиля,
        // а посмотреть, кого уже заблокировал, было негде.
        { path: '/blocked', element: <BlockedPage /> },
        { path: '/notifications', element: <NotificationsPage /> },
        // Админка ПВЗ живёт в том же кабинете: у сотрудника это рабочее место, а не
        // отдельный продукт. Роут открыт всем, кто вошёл, — доступ решает бэкенд по роли.
        { path: '/admin/deliveries', element: <AdminDeliveriesPage /> },
        // Модерация — там же и на тех же правах: очередь жалоб, разбор одной и журнал.
        { path: '/admin/moderation', element: <AdminModerationPage /> },
        { path: '/admin/moderation/:reportId', element: <AdminReportPage /> },
        { path: '/admin/audit', element: <AdminAuditPage /> },
        // Вложенные экраны: сюда ведут из разделов, путь назад показывают крошки.
        { path: '/items/new', element: <CreateOfferPage /> },
        { path: '/items/:id/barter', element: <EnableBarterPage /> },
        { path: '/items/:id/edit', element: <EditItemPage /> },
        // Свой профиль и чужой — один экран: у своего есть правка, у чужого только просмотр.
        // Идентификатор своего в адресе не нужен, он приходит из сессии.
        { path: '/profile', element: <ProfilePage mine /> },
        { path: '/users/:id', element: <ProfilePage /> },
        { path: '/exchange/:id', element: <ChainPage /> },
        // Переписка адресуется парой «цепочка + собеседник»: она привязана к ребру круга
        // обмена, а не к вещи — с одним человеком в двух цепочках это два разных разговора.
        { path: '/messages/:itemId/:counterpartId', element: <ThreadPage /> },
      ],
    },
  ],
  // На GitHub Pages сайт живёт в подпапке репозитория — роутер должен знать префикс,
  // иначе все ссылки уедут в корень домена. Значение подставляет Vite из `base`.
  { basename: import.meta.env.BASE_URL },
)
