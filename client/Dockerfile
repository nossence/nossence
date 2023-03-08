FROM node:lts-alpine3.17 AS builder

WORKDIR /build

COPY package.json .

RUN npm i

COPY . .

ARG NEXT_PUBLIC_API_BASE_URI

RUN npm run build

FROM node:lts-alpine3.17

WORKDIR /app

COPY package.json .

RUN npm i --production

COPY --from=builder /build/.next /app/.next

EXPOSE 3000

ENTRYPOINT npm start
