FROM node:22-alpine AS build

WORKDIR /app

ARG VITE_DATA_SOURCE=api
ARG VITE_API_BASE_URL=
ENV VITE_DATA_SOURCE=${VITE_DATA_SOURCE}
ENV VITE_API_BASE_URL=${VITE_API_BASE_URL}

COPY website/package.json website/package-lock.json ./
RUN npm ci --no-audit --no-fund

COPY website/ ./
RUN npm run build

FROM nginx:1.27-alpine

COPY deploy/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html

EXPOSE 80
