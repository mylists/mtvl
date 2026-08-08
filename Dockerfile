FROM golang:alpine AS build

COPY / /app

WORKDIR /app

RUN go build -o /out .

#################################################################

FROM alpine:latest AS deploy

COPY --link --from=build /out /out

ENTRYPOINT [ "/out" ]
