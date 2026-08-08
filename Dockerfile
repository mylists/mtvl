FROM alpine:latest AS build

RUN apk update && \
  apk add go

COPY / /app

WORKDIR /app

RUN go build -o /out .

#################################################################

FROM alpine:latest AS deploy

COPY --link --from=build /out /out

ENTRYPOINT [ "/out" ]
