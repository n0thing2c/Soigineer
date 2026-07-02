FROM node:22-alpine AS build

WORKDIR /app

RUN corepack enable

COPY package.json pnpm-lock.yaml ./

RUN pnpm install --frozen-lockfile

COPY index.html postcss.config.js tailwind.config.ts tsconfig.json vite.config.ts ./
COPY public ./public
COPY client ./client

ARG VITE_AUTH_API_URL=http://localhost:8070/v1
ARG VITE_MONITORING_API_URL=http://localhost:8090/v1

ENV VITE_AUTH_API_URL=$VITE_AUTH_API_URL
ENV VITE_MONITORING_API_URL=$VITE_MONITORING_API_URL

RUN pnpm build

FROM nginx:1.27-alpine AS runtime

COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html

EXPOSE 80

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://127.0.0.1/ || exit 1