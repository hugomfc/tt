# Build stage
FROM golang:1.17-alpine AS build

WORKDIR /app

COPY go.mod .
RUN go mod download

COPY . .
RUN go build -o tt ./cmd/tt

# Production stage
FROM alpine:3.14

WORKDIR /app

COPY --from=build /app/tt /app/tt
RUN chmod +x /app/tt

EXPOSE 8080

CMD ["/app/tt"]

