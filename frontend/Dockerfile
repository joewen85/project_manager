ARG NODE_BUILDER_IMAGE=node:22-alpine
ARG NGINX_IMAGE=registry.cn-guangzhou.aliyuncs.com/joe/nginx:alpine
ARG NPM_REGISTRY=https://registry.npmmirror.com

FROM ${NODE_BUILDER_IMAGE} AS builder
ARG NPM_REGISTRY
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm config set registry "${NPM_REGISTRY}" && npm install
COPY . .
RUN npm run build

FROM ${NGINX_IMAGE}
COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/templates/default.conf.template
EXPOSE 80
EXPOSE 443
CMD ["nginx", "-g", "daemon off;"]
