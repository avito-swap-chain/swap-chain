import { notify } from '@/shared/model/notifications'
import { currentPersonaId, PERSONAS, type Persona } from '@/shared/model/persona'
import { confirmReceiptFor } from '../lib/participants'
import type { Chain, ChainDecision, ChainParticipant, ParticipantStatus } from '../model/types'

/**
 * Демо-данные цепочек и операции над ними: то, что делал бы бэкенд, если бы он был подключён.
 * Лежат отдельно от `chainApi`, чтобы в нём были видны только реальные вызовы — иначе полторы
 * сотни строк заготовленных обменов заслоняют пять запросов, ради которых файл и существует.
 *
 * Демо на моках должно открываться по ссылке без поднятого бэкенда, поэтому это не тестовый
 * стенд, а часть продукта: сценарии здесь подобраны так, чтобы каждый экран было чем показать.
 */

const [DASHA, MARK, LENA] = PERSONAS

/** Участник-персона: имя, рейтинг и аватар берём из общего реестра, чтобы они не разъезжались. */
const of = (
  persona: Persona,
  givesItem: ChainParticipant['givesItem'],
  status: ParticipantStatus,
  receiptConfirmed = false,
): ChainParticipant => ({
  userId: persona.id,
  name: persona.name,
  avatarUrl: persona.avatarUrl,
  rating: persona.rating,
  givesItem,
  status,
  receiptConfirmed,
})

// Порядок участников = обход цикла: participants[i] отдаёт вещь participants[i + 1].
// Флага `isMe` в данных нет: чья это цепочка — вопрос точки зрения, см. `asSeenBy`.
// Длина цепочки — три участника: по исследованиям обмена почками переход с трёх на четыре
// добавляет 1–5% совпадений, а алгоритм и объяснимость сделки тяжелеют заметно сильнее.
// Подбор параллельный: у Даши сразу три предложения, и два из них держат один велосипед —
// состоится ровно одно, второе отменится. Это штатный ход сервиса, а не крайний случай.
let chains: Chain[] = [
  // Демонстрационная цепочка: все трое — персоны, поэтому один обмен виден тремя парами глаз.
  // Даша отдаёт велосипед Марку, Марк наушники — Лене, Лена фотоаппарат — Даше: каждый получает
  // ровно то, что указал в желании (см. `wish` этих вещей в `itemMocks`).
  {
    id: 'c1',
    status: 'formed',
    participants: [
      of(
        DASHA,
        { id: '1', title: 'Горный велосипед', photoUrl: '/mock/items/bike.jpg' },
        'pending',
      ),
      of(
        MARK,
        { id: '21', title: 'Наушники', photoUrl: '/mock/items/headphones.jpg' },
        'confirmed',
      ),
      of(
        LENA,
        { id: '31', title: 'Плёночный фотоаппарат', photoUrl: '/mock/items/camera.jpg' },
        'pending',
      ),
    ],
  },
  // Соперник c1 за тот же велосипед: Даша получает приставку — второй вариант её желания
  // (первый закрывает c1). Двое уже лайкнули, поэтому её лайк собирает цепочку сразу,
  // и на демо видно, как конкурирующее предложение отменяется.
  {
    id: 'c4',
    status: 'formed',
    participants: [
      of(
        DASHA,
        { id: '1', title: 'Горный велосипед', photoUrl: '/mock/items/bike.jpg' },
        'pending',
      ),
      {
        userId: 'u9',
        name: 'Костя',
        avatarUrl: '/mock/avatars/u15.jpg',
        rating: 4.7,
        givesItem: { id: '91', title: 'Смартфон Redmi Note 12', photoUrl: '/mock/items/phone.jpg' },
        status: 'confirmed',
        receiptConfirmed: false,
      },
      {
        userId: 'u10',
        name: 'Аня',
        avatarUrl: '/mock/avatars/u5.jpg',
        rating: 4.9,
        givesItem: {
          id: '92',
          title: 'Игровая приставка PlayStation 4',
          photoUrl: '/mock/items/console.jpg',
        },
        status: 'confirmed',
        receiptConfirmed: false,
      },
    ],
  },
  // Прямой обмен на двоих — вырожденная цепочка, но такая же законная. Держит другую вещь
  // Даши (гантели), поэтому от исхода велосипедных вариантов не зависит.
  {
    id: 'c5',
    status: 'formed',
    participants: [
      of(DASHA, { id: '2', title: 'Гантели 20 кг' }, 'pending'),
      {
        userId: 'u11',
        name: 'Петя',
        rating: 4.5,
        givesItem: {
          id: '93',
          title: 'Умные часы Xiaomi Watch S3',
          photoUrl: '/mock/items/watch.jpg',
        },
        status: 'pending',
        receiptConfirmed: false,
      },
    ],
  },
  // Идущая передача: Игорь свою вещь уже получил, Соня — ещё нет. Значит после отметки
  // текущего пользователя цепочка не закроется — видно состояние «жду остальных».
  {
    id: 'c2',
    status: 'active',
    participants: [
      of(DASHA, { id: '5', title: 'Кофеварка', photoUrl: '/mock/items/coffee.jpg' }, 'confirmed'),
      {
        userId: 'u5',
        name: 'Игорь',
        avatarUrl: '/mock/avatars/u60.jpg',
        rating: 4.8,
        givesItem: { id: '51', title: 'Акустическая гитара', photoUrl: '/mock/items/guitar.jpg' },
        status: 'confirmed',
        receiptConfirmed: true,
      },
      // Единственный участник без `avatarUrl` — намеренно: на нём видно фолбэк на инициал.
      {
        userId: 'u6',
        name: 'Соня',
        rating: 4.6,
        givesItem: { id: '61', title: 'Электросамокат', photoUrl: '/mock/items/scooter.jpg' },
        status: 'confirmed',
        receiptConfirmed: false,
      },
    ],
  },
  {
    id: 'c3',
    status: 'completed',
    participants: [
      of(
        LENA,
        { id: '32', title: 'Настольная лампа', photoUrl: '/mock/items/lamp.jpg' },
        'confirmed',
        true,
      ),
      {
        userId: 'u7',
        name: 'Паша',
        avatarUrl: '/mock/avatars/u68.jpg',
        rating: 4.9,
        givesItem: { id: '71', title: 'Монитор 24"', photoUrl: '/mock/items/monitor24.jpg' },
        status: 'confirmed',
        receiptConfirmed: true,
      },
      {
        userId: 'u8',
        name: 'Катя',
        avatarUrl: '/mock/avatars/u26.jpg',
        rating: 5.0,
        givesItem: { id: '81', title: 'Плед', photoUrl: '/mock/items/blanket.jpg' },
        status: 'confirmed',
        receiptConfirmed: true,
      },
    ],
  },
]

const delay = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms))

function find(id: string): Chain {
  const chain = chains.find((c) => c.id === id)
  if (!chain) throw new Error(`Цепочка ${id} не найдена`)
  return chain
}

/**
 * Проверка перед изменением: `replace` молча ничего не делает с несуществующей цепочкой,
 * а действие над чужим или удалённым id должно падать — так же, как ответил бы бэкенд.
 */
const requireChain = (id: string): void => void find(id)

/** Заменить цепочку новым объектом — ссылка меняется, TanStack Query видит обновление. */
function replace(id: string, update: (chain: Chain) => Chain) {
  chains = chains.map((chain) => (chain.id === id ? update(chain) : chain))
}

/**
 * Цепочка глазами конкретного пользователя: `isMe` не лежит в данных, а проставляется
 * на ответе — ровно так же поступил бы бэкенд, зная, кто именно запрашивает.
 */
const asSeenBy = (chain: Chain, personaId: string): Chain => ({
  ...chain,
  participants: chain.participants.map((p) => (p.userId === personaId ? { ...p, isMe: true } : p)),
})

const withPersonaStatus = (chain: Chain, personaId: string, status: ParticipantStatus): Chain => ({
  ...chain,
  participants: chain.participants.map((p) => (p.userId === personaId ? { ...p, status } : p)),
})

/**
 * Цепочка собралась — её вещи заблокированы, и конкурирующие предложения с теми же вещами
 * собрать уже нельзя. Отменяем их сразу, запоминая вещь-причину: без неё человеку не объяснить,
 * почему предложение, которое он только что видел, больше не работает.
 */
function cancelRivals(formedId: string) {
  const taken = new Set(find(formedId).participants.map((p) => p.givesItem.id))

  chains = chains.map((chain) => {
    if (chain.id === formedId || chain.status !== 'formed') return chain

    const lost = chain.participants.find((p) => taken.has(p.givesItem.id))
    if (!lost) return chain

    const cancelled: Chain = { ...chain, status: 'cancelled', cancelledItemId: lost.givesItem.id }
    // Отмену человек увидит и в списке, но узнать о ней он должен, даже если ушёл с экрана.
    if (cancelled.participants.some((p) => p.userId === currentPersonaId())) {
      notify({
        kind: 'offer',
        title: 'Вариант обмена отменён',
        text: `«${lost.givesItem.title}» ушла в другую цепочку. Остальные ваши варианты в силе.`,
        to: '/exchange',
      })
    }
    return cancelled
  })
}

/** Обмены, в которых участвует текущая персона, — чужие в кабинет не попадают. */
export async function myChains(): Promise<Chain[]> {
  await delay(300)
  const personaId = currentPersonaId()

  return chains
    .filter((chain) => chain.participants.some((p) => p.userId === personaId))
    .map((chain) => asSeenBy(chain, personaId))
}

export async function chainById(id: string): Promise<Chain> {
  await delay(250)
  return asSeenBy(find(id), currentPersonaId())
}

export async function respond(id: string, decision: ChainDecision): Promise<void> {
  await delay(400)
  requireChain(id)
  const personaId = currentPersonaId()

  if (decision === 'dislike') {
    replace(id, (chain) => ({
      ...withPersonaStatus(chain, personaId, 'declined'),
      status: 'dissolved',
    }))
    return
  }

  replace(id, (chain) => withPersonaStatus(chain, personaId, 'confirmed'))

  // Демо-имитация лайков остальных участников: на бэке это придёт обычным refetch.
  setTimeout(() => {
    const chain = find(id)
    // За это время предложение могло отмениться, а пользователь — выйти из цепочки.
    if (chain.status !== 'formed' || chain.participants.some((p) => p.status === 'declined')) return

    replace(id, (current) => ({
      ...current,
      status: 'active',
      participants: current.participants.map((p) => ({ ...p, status: 'confirmed' })),
    }))
    notify({
      kind: 'chain',
      title: 'Цепочка собралась',
      text: 'Все участники согласны. Сдайте вещь в пункт выдачи и отметьте, когда получите свою.',
      to: `/exchange/${id}`,
    })
    cancelRivals(id)
  }, 2500)
}

export async function leave(id: string): Promise<void> {
  await delay(400)
  requireChain(id)
  replace(id, (chain) => ({
    ...withPersonaStatus(chain, currentPersonaId(), 'declined'),
    status: 'dissolved',
  }))
}

export async function confirmReceipt(id: string): Promise<void> {
  await delay(400)
  requireChain(id)
  replace(id, (chain) => confirmReceiptFor(chain, currentPersonaId()))

  // Демо-имитация отметки остальных участников: на бэке это придёт обычным refetch.
  setTimeout(() => {
    // За это время получение мог отметить последний участник — закрытую цепочку не трогаем.
    if (find(id).status !== 'active') return

    replace(id, (chain) => ({
      ...chain,
      status: 'completed',
      participants: chain.participants.map((p) => ({ ...p, receiptConfirmed: true })),
    }))
    notify({
      kind: 'chain',
      title: 'Обмен завершён',
      text: 'Все участники подтвердили получение вещей.',
      to: `/exchange/${id}`,
    })
  }, 2500)
}

/**
 * Вещь сняли с обмена — предложения с ней больше не соберутся. На бэкенде это делает он сам
 * и присылает `chain.rejected` с причиной `item_withdrawn`; здесь распускаем сами, иначе
 * в демо остались бы висеть варианты с вещью, которой в подборе уже нет.
 */
export function dissolveWithItem(itemId: string) {
  chains = chains.map((chain) =>
    chain.status === 'formed' && chain.participants.some((p) => p.givesItem.id === itemId)
      ? { ...chain, status: 'dissolved' }
      : chain,
  )
}
