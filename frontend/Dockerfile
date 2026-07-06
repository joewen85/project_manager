ARG NODE_BUILDER_IMAGE=registry.cn-guangzhou.aliyuncs.com/joe/node:24-alpine
ARG NGINX_IMAGE=registry.cn-guangzhou.aliyuncs.com/joe/nginx:alpine
ARG NPM_REGISTRY=https://registry.npmmirror.com

FROM ${NODE_BUILDER_IMAGE} AS builder
ARG NPM_REGISTRY
ARG VITE_API_BASE_URL=/api/v1
ARG VITE_AI_ASSISTANT_TIMEOUT_MS=35000
ENV VITE_API_BASE_URL=${VITE_API_BASE_URL}
ENV VITE_AI_ASSISTANT_TIMEOUT_MS=${VITE_AI_ASSISTANT_TIMEOUT_MS}
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm config set registry "${NPM_REGISTRY}" && npm install
COPY . .
RUN npm run build

FROM ${NGINX_IMAGE}
COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/nginx.conf
COPY app.conf /etc/nginx/templates/default.conf.template
EXPOSE 80
EXPOSE 443
CMD ["nginx", "-g", "daemon off;"]
