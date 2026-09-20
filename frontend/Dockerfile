FROM node:20-alpine AS build
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm install
COPY . .
RUN npm run build

FROM nginx:1.27-alpine
COPY --from=build /app/dist /usr/share/nginx/html
COPY nginx.conf.template /etc/nginx/templates/default.conf.template
COPY docker-entrypoint.d/10-set-resolver.envsh /docker-entrypoint.d/10-set-resolver.envsh
RUN chmod +x /docker-entrypoint.d/10-set-resolver.envsh
ENV BACKEND_HOST=backend BACKEND_PORT=8000 BACKEND_SCHEME=http PORT=80
EXPOSE 80