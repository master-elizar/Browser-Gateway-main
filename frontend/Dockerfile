# syntax=docker/dockerfile:1
FROM node:22-bookworm AS build
WORKDIR /app
COPY package.json ./
RUN npm install
COPY . .
RUN npm run build

FROM nginx:1.27-alpine
RUN apk add --no-cache curl \
  && curl -fsSL https://github.com/novnc/noVNC/archive/refs/tags/v1.5.0.tar.gz \
    | tar xz -C /tmp \
  && mv /tmp/noVNC-1.5.0 /usr/share/nginx/html/novnc
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html
RUN if [ -f /usr/share/nginx/html/novnc-embed.html ]; then \
      mv /usr/share/nginx/html/novnc-embed.html /usr/share/nginx/html/novnc/embed.html; \
    fi
EXPOSE 80
