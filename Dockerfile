# Build stage for frontend
FROM node:20-alpine AS frontend-build
WORKDIR /app/frontend
COPY frontend/package*.json frontend/pnpm-lock.yaml ./
RUN npm install -g pnpm && pnpm install
COPY frontend/ ./
RUN pnpm build

# Build stage for backend
FROM golang:1.24-alpine AS backend-build
WORKDIR /app/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -o dimagram .

# Final stage
FROM alpine:latest
WORKDIR /app
RUN apk --no-cache add ca-certificates

# Copy built artifacts
COPY --from=backend-build /app/backend/dimagram /app/
COPY --from=frontend-build /app/frontend/dist /app/frontend

# Environment variables
ENV PORT=8080 

# Expose port
EXPOSE 8080

ENTRYPOINT ["/app/dimagram"]
CMD ["server"]
