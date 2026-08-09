/**
 * Снимает экраны демо со свежего фронта в public/screens.
 *
 *   pnpm --dir ../avito-chain dev     # поднять фронт
 *   node shoot.mjs [http://localhost:5173]
 *
 * Скрипт не запускает фронт сам: dev-сервер живёт дольше съёмки и его удобнее
 * держать отдельно. Персона переключается через реальный <select> в шапке —
 * так же, как это делает человек, поэтому снимок совпадает с тем, что увидит жюри.
 */
import { chromium } from 'playwright'
import { mkdir } from 'node:fs/promises'

const BASE = process.argv[2] ?? 'http://localhost:5173'
const OUT = new URL('./public/screens/', import.meta.url)

/** Экраны в порядке сценария из src/scenes.ts. */
const SHOTS = [
  { file: '01-items.png', path: '/' },
  { file: '02-new-item.png', path: '/items/new' },
  // Распознавание по фото — три кадра одного шага: пусто, идёт разбор, поля заполнены.
  { file: '03-ai-thinking.png', path: '/items/new', upload: 'bike.jpg', waitMs: 700 },
  { file: '04-ai-done.png', path: '/items/new', upload: 'bike.jpg', waitMs: 2600 },
  { file: '05-enable-barter.png', path: '/items/3/barter' },
  { file: '06-offers.png', path: '/exchange' },
  { file: '07-chain-offer.png', path: '/exchange/c1' },
  { file: '08-chain-waiting.png', path: '/exchange/c1', persona: 'u2' },
  { file: '09-chain-handoff.png', path: '/exchange/c2' },
  { file: '10-chain-completed.png', path: '/exchange/c3', persona: 'u3' },
  { file: '11-chain-dissolved.png', path: '/exchange/c1', decline: true },
]

/** Переключение персоны: React слушает change, поэтому значение ставим нативным сеттером. */
function setPersona(id) {
  const select = document.querySelector('header select')
  const setter = Object.getOwnPropertyDescriptor(HTMLSelectElement.prototype, 'value').set
  setter.call(select, id)
  select.dispatchEvent(new Event('change', { bubbles: true }))
}

async function main() {
  await mkdir(OUT, { recursive: true })

  const browser = await chromium.launch()
  const page = await browser.newPage({
    // Ниже реального телефона: в кадре ролика вытянутая рамка выглядит неестественно,
    // а экраны у нас всё равно короче — прокрутка нужна только списку предложений.
    viewport: { width: 430, height: 800 },
    deviceScaleFactor: 2,
  })

  for (const shot of SHOTS) {
    await page.goto(BASE + shot.path, { waitUntil: 'networkidle' })

    if (shot.persona) {
      await page.evaluate(setPersona, shot.persona)
      await page.waitForTimeout(600)
    }

    if (shot.upload) {
      // Playwright сам кликает по input[type=file] — снимаем ровно то, что видит человек.
      await page.setInputFiles('input[type=file]', new URL(`../avito-chain/public/mock/items/${shot.upload}`, import.meta.url).pathname)
      await page.waitForTimeout(shot.waitMs ?? 800)
    }

    if (shot.decline) {
      await page
        .getByRole('button', { name: /Отклонить|Дизлайк|Не подходит/i })
        .first()
        .click()
      await page.waitForTimeout(1200)
    }

    await page.waitForTimeout(400)

    // Страница растянута на всю высоту окна (`min-h-svh`), поэтому fullPage добавляет пустоту
    // под контентом: на коротком экране она превращалась бы в прокрутку по белому полю.
    // Режем по нижней границе реального содержимого.
    const height = await page.evaluate(() => {
      let bottom = 0
      for (const el of document.body.querySelectorAll('*')) {
        const box = el.getBoundingClientRect()
        if (box.width > 0 && box.height > 0) bottom = Math.max(bottom, box.bottom + window.scrollY)
      }
      return Math.ceil(bottom) + 16
    })

    await page.screenshot({
      path: new URL(shot.file, OUT).pathname,
      clip: { x: 0, y: 0, width: 430, height: Math.max(height, 400) },
    })
    console.log('снято', shot.file, `${height}px`)
  }

  await browser.close()
}

main()
