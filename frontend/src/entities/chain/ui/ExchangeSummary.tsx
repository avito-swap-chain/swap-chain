import { cx, dative, genitive } from '@/shared/lib'
import { IconBox } from '@/shared/ui'
import type { Neighbours } from '../lib/participants'
import type { ChainParticipant } from '../model/types'
import { ParticipantAvatar } from './ParticipantAvatar'

type ItemRef = ChainParticipant['givesItem']

interface ExchangeSummaryProps {
  /** Участник, за которого играет текущий пользователь: его вещь и уходит в цепочку. */
  me: ChainParticipant
  /** Соседи: вещь получает `receiver`, а приходит она от `giver`. */
  neighbours: Neighbours
  /** Обмен уже состоялся — подписи в прошедшем времени. */
  past?: boolean
}

/**
 * Эгоцентричная суть обмена: «получаете Y от Ани, отдаёте X Марку».
 * Первое, что видит человек, — полная цепочка идёт вторым слоем (гипотеза H1).
 *
 * Блок не залит цветом: это главный контент экрана, ему не нужна подложка, чтобы
 * быть заметным, а азур в системе Авито закреплён за кнопками и ссылками.
 * Порядок и вес строк держат иерархию: выгода («получаете») крупнее и стоит первой,
 * цена обмена («отдаёте») читается сразу под ней, но тише.
 *
 * Имена соседей стоят здесь, а не только в ленте: главный вопрос человека — с кем он
 * имеет дело, ведь встречаться и передавать вещь он будет именно с ними.
 */
export function ExchangeSummary({ me, neighbours, past = false }: ExchangeSummaryProps) {
  const { receiver, giver } = neighbours

  return (
    <div>
      <Row
        item={giver.givesItem}
        caption={past ? 'Получили от' : 'Получаете от'}
        party={giver}
        name={genitive(giver.name)}
        lead
      />
      <Row
        item={me.givesItem}
        caption={past ? 'Отдали' : 'Отдаёте'}
        party={receiver}
        name={dative(receiver.name)}
      />
    </div>
  )
}

/**
 * Одна сторона обмена: вещь и человек, с которым она меняет владельца.
 * `lead` — сторона, ради которой человек в цепочке: крупнее миниатюра и название.
 */
function Row({
  item,
  caption,
  party,
  name,
  lead = false,
}: {
  item: ItemRef
  caption: string
  party: ChainParticipant
  name: string
  lead?: boolean
}) {
  return (
    <div className={cx('flex items-center gap-3 py-2.5', !lead && 'border-t border-line')}>
      <Thumb item={item} className={lead ? 'size-14' : 'size-11'} />

      <div className="min-w-0">
        <span className="flex items-center gap-1 text-[12px] leading-4 text-ink-2">
          {caption}
          <ParticipantAvatar
            participant={party}
            className="size-4 bg-line-2 text-[9px] font-bold text-ink-2"
          />
          <span className="min-w-0 truncate font-bold">{name}</span>
        </span>
        <b className={cx('block', lead ? 'text-[16.5px] font-bold' : 'text-[14px] font-normal')}>
          {item.title}
        </b>
      </div>
    </div>
  )
}

/** Миниатюра вещи; без фото — коробка-плейсхолдер, как в объявлении без снимка. */
function Thumb({ item, className }: { item: ItemRef; className: string }) {
  const box = cx('shrink-0 rounded-xl bg-line-2', className)

  if (!item.photoUrl) {
    return (
      <span className={cx('grid place-items-center text-ink-3', box)}>
        <IconBox size={24} />
      </span>
    )
  }

  return <img src={item.photoUrl} alt="" loading="lazy" className={cx('object-cover', box)} />
}
