import { Config } from '@remotion/cli/config'

Config.setVideoImageFormat('jpeg')
Config.setOverwriteOutput(true)
// Демо смотрят в телеграме с телефона — вертикальный кадр читается лучше горизонтального.
Config.setEntryPoint('./src/index.ts')
