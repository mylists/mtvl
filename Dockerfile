FROM golang:1.26-alpine AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out .

#################################################################

FROM alpine:latest AS deploy

COPY --link --from=build /out /out

ENTRYPOINT [ "/out" ]
