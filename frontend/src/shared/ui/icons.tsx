import type { ReactNode } from 'react'

interface IconProps {
  size?: number
  className?: string
}

function Icon({
  size = 16,
  strokeWidth = 2,
  className,
  children,
}: IconProps & { strokeWidth?: number; children: ReactNode }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={strokeWidth}
      strokeLinecap="round"
      strokeLinejoin="round"
      className={className}
      aria-hidden="true"
    >
      {children}
    </svg>
  )
}

export const IconCheck = (props: IconProps) => (
  <Icon strokeWidth={3} {...props}>
    <path d="M5 12l5 5 9-11" />
  </Icon>
)

export const IconClock = (props: IconProps) => (
  <Icon strokeWidth={2.2} {...props}>
    <circle cx="12" cy="12" r="8" />
    <path d="M12 8v4l3 2" />
  </Icon>
)

export const IconClose = (props: IconProps) => (
  <Icon strokeWidth={2.4} {...props}>
    <path d="M6 6l12 12M18 6L6 18" />
  </Icon>
)

export const IconPlus = (props: IconProps) => (
  <Icon strokeWidth={3} {...props}>
    <path d="M12 5v14M5 12h14" />
  </Icon>
)

export const IconArrowRight = (props: IconProps) => (
  <Icon {...props}>
    <path d="M5 12h13M13 6l6 6-6 6" />
  </Icon>
)

export const IconChevronLeft = (props: IconProps) => (
  <Icon {...props}>
    <path d="M15 5l-7 7 7 7" />
  </Icon>
)

export const IconImage = (props: IconProps) => (
  <Icon strokeWidth={1.5} {...props}>
    <rect x="3" y="5" width="18" height="14" rx="2" />
    <circle cx="8.5" cy="10" r="1.5" />
    <path d="M3 16l5-4 4 3 3-2 6 5" />
  </Icon>
)

/** Плейсхолдер вещи, пока нет фото. */
export const IconBox = (props: IconProps) => (
  <Icon strokeWidth={1.5} {...props}>
    <path d="M3 8l9-5 9 5v8l-9 5-9-5z" />
    <path d="M3 8l9 5 9-5M12 13v8" />
  </Icon>
)
