FROM node:22-bookworm-slim AS deps

WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci

FROM node:22-bookworm-slim AS builder

WORKDIR /app

COPY --from=deps /app/node_modules ./node_modules
COPY . .

ARG EXPO_PUBLIC_API_BASE=http://localhost:8080/api/v1
ENV EXPO_PUBLIC_API_BASE=${EXPO_PUBLIC_API_BASE}

RUN npx expo export --platform web --output-dir dist

FROM nginx:1.27-alpine

COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /app/dist /usr/share/nginx/html

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
