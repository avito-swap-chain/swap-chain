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
RUN pnpm build

# --- serve stage ---
FROM nginx:1.27-alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
