import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { PERSONAS, usePersonaStore } from '@/shared/model/persona'
import {
  DEMO_USERS,
  getCurrentUser,
  knownAccounts,
  login,
  logout,
  sessionKeys,
  type CurrentUser,
} from '@/shared/model/session'
import { asset, cx } from '@/shared/lib'
import { isBackendConnected } from '@/shared/config/backend'
import { ActionError, IconCheck } from '@/shared/ui'

/** Строка переключателя: имя в поле, телефон для входа, корона у сотрудника ПВЗ. */
interface SwitchOption {
  name: string
  phone: string
  isAdmin?: boolean
}

/** Разделы кабинета — как в меню профиля у Авито: из шапки видно, куда идти. */
const SECTIONS = [
  { label: 'Мои объявления', to: '/' },
  { label: 'Обмен', to: '/exchange' },
  { label: 'Сообщения', to: '/messages' },
  { label: 'Уведомления', to: '/notifications' },
]

/**
 * Аватар аккаунта — крайний правый элемент шапки, как у Авито (40px, круглый).
 * Он же показывает, за кого сейчас смотрят: имя в меню дублируется лицом. У пользователя
 * с бэкенда фотографии нет — тогда кружок с инициалом, выдумывать ему лицо незачем.
 *
 * Админа помечаем короной: на демо переключаются между обычным человеком и сотрудником ПВЗ,
 * и по одному имени не понять, почему у одного аккаунта вдруг появились чужие доставки.
 */
function AccountAvatar({ user }: { user: CurrentUser }) {
  const face = user.avatarUrl ? (
    <img
      src={asset(user.avatarUrl)}
      alt=""
      width={40}
      height={40}
      className="size-10 rounded-full object-cover"
    />
  ) : (
    <span
      aria-hidden
      className="grid size-10 place-items-center rounded-full bg-line text-[15px] font-bold text-ink-2"
    >
      {user.name.slice(0, 1).toUpperCase()}
    </span>
  )

  if (!user.isAdmin) return face

  return (
    <span className="relative block">
      {face}
      {/* Корона садится на угол аватара, но не вылезает за ярус шапки: выше ей места нет. */}
      <span aria-hidden className="absolute -top-0.5 -right-1 text-[13px] leading-none">
        👑
      </span>
    </span>
  )
}

/**
 * Демо-инструмент: посмотреть один и тот же обмен глазами разных участников — иначе
 * цепочку из трёх человек по ссылке не показать.
 *
 * Живёт там же, где у Авито меню аккаунта: аватар в шапке, по клику — карточка со списком.
 * Сверху «Смотрю как» (этого пункта у Авито нет, но и персон у него нет), ниже разделы
 * кабинета и выход — как у них.
 *
 * На моках переключатель только меняет, кто «я»: в бизнес-логику не вмешивается, `isMe`
 * и состав кабинета считает api. С бэкендом смотреть чужими глазами можно только
 * по-настоящему — пользователя опознаёт кука, поэтому переключение это выход и вход
 * под другим телефоном.
 */
export function SwitchPersona() {
  const [open, setOpen] = useState(false)
  const personaId = usePersonaStore((state) => state.personaId)
  const setPersonaId = usePersonaStore((state) => state.setPersonaId)
  const queryClient = useQueryClient()
  const { data: user } = useQuery({ queryKey: sessionKeys.current(), queryFn: getCurrentUser })

  const switchUser = useMutation({
    mutationFn: async (phone: string) => {
      await logout()
      await login(phone)
    },
    // В кеше лежат ответы прошлого пользователя — без сброса экран показал бы чужой кабинет.
    onSettled: () => queryClient.resetQueries(),
  })

  if (!user) return null

  // Список для выбора: засеянные бэкендом демо-пользователи плюс все, кто входил с этого
  // браузера. Раньше своим здесь был только текущий, и после переключения на другого человека
  // зарегистрировавшийся терял себя из виду. Сравниваем по телефону: имена не уникальны,
  // и на них выбор промахивался.
  const options: SwitchOption[] = isBackendConnected
    ? [
        ...knownAccounts().filter(
          (account) => !DEMO_USERS.some((demo) => demo.phone === account.phone),
        ),
        ...DEMO_USERS,
      ]
    : PERSONAS.map((persona) => ({ name: persona.name, phone: persona.id }))

  const currentValue = isBackendConnected ? (user.phone ?? '') : personaId

  const choose = (value: string) => {
    setOpen(false)

    if (!isBackendConnected) {
      setPersonaId(value)
      queryClient.resetQueries()
      return
    }

    switchUser.mutate(value)
  }

  return (
    <div
      /* `shrink-0` обязателен: верхний ярус шапки на узком десктопе переполняется,
         и без него флекс сжимал аватар до нуля — переключатель персон пропадал
         в диапазоне 900–1150px, а другого входа в него на этих ширинах нет. */
      className="relative flex shrink-0 pl-2"
      onKeyDown={(event) => {
        if (event.key === 'Escape') setOpen(false)
      }}
    >
      <button
        type="button"
        aria-haspopup="menu"
        aria-expanded={open}
        aria-label={user.isAdmin ? `${user.name} — сотрудник ПВЗ` : user.name}
        onClick={() => setOpen((was) => !was)}
        className="cursor-pointer rounded-full outline-offset-2 focus-visible:outline-2 focus-visible:outline-brand"
      >
        <AccountAvatar user={user} />
      </button>

      {/* Переключение — это настоящие выход и вход, и оно тоже может не получиться.
          Сообщение висит под шапкой: в самой шапке для него нет ряда. */}
      {switchUser.isError && (
        <span className="absolute top-full right-0 z-30 mt-1 w-max rounded-card bg-card px-2.5 py-1.5 shadow-pop">
          <ActionError error={switchUser.error} />
        </span>
      )}

      {open && (
        /* Подложка ловит клик мимо меню: без неё карточка залипает поверх страницы.
           Она выше плавающего мессенджера — иначе клик по нему меню не закрывает. */
        <div className="fixed inset-0 z-40" onClick={() => setOpen(false)} aria-hidden />
      )}

      <div className={cx('absolute top-full right-0 z-50 pt-1.5', !open && 'hidden')}>
        <div className="w-[221px] rounded-[24px] bg-card py-2 text-[15px] leading-[22px] shadow-pop">
          <div role="listbox" aria-label="Смотрю как" className="px-6 py-4">
            <p className="pb-1.5 text-[13px] leading-4 text-ink-3">Смотрю как</p>
            {options.map((option) => {
              const selected = option.phone === currentValue

              return (
                <button
                  key={option.phone || option.name}
                  type="button"
                  role="option"
                  aria-selected={selected}
                  disabled={switchUser.isPending}
                  onClick={() => choose(option.phone)}
                  className="flex w-full cursor-pointer items-center justify-between gap-2 py-1 text-left hover:text-brand"
                >
                  {/* Корона и в списке — иначе на демо не видно, под кем открывать админку ПВЗ. */}
                  {option.isAdmin ? `${option.name} 👑` : option.name}
                  {selected && <IconCheck size={14} className="shrink-0 text-brand" />}
                </button>
              )
            })}
          </div>

          <div className="border-t border-line px-6 py-4">
            {SECTIONS.map((section) => (
              <Link
                key={section.to}
                to={section.to}
                onClick={() => setOpen(false)}
                className="block py-1 hover:text-brand"
              >
                {section.label}
              </Link>
            ))}
          </div>

          {isBackendConnected && (
            <div className="border-t border-line px-6 py-4">
              <button
                type="button"
                onClick={() => {
                  setOpen(false)
                  logout().then(() => queryClient.resetQueries())
                }}
                className="cursor-pointer py-1 hover:text-brand"
              >
                Выйти
              </button>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
