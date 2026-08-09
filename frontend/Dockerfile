# --- build stage ---
FROM node:22-alpine AS build
WORKDIR /app
RUN corepack enable
# pnpm-workspace.yaml обязателен рядом с манифестом: в pnpm 11 оттуда читаются
# настройки проекта — разрешение сборочных скриптов (allowBuilds) и minimumReleaseAge.
# Без него install в чистом образе падает на политике свежести пакетов.
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile
COPY . .

# Куда фронт ходит за данными. По умолчанию `/` — тот же адрес, что и сам фронт:
# nginx рядом проксирует /api на бэкенд. Пустое значение соберёт демо на мок-данных.
ARG VITE_API_URL=/
ENV VITE_API_URL=$VITE_API_URL
RUN pnpm build

# --- serve stage ---
FROM nginx:1.27-alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
