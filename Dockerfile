FROM alpine:latest AS build

RUN apk update && \
  apk add go

COPY / /app

RUN go build -o /out /app

#################################################################

FROM alpine:latest AS deploy

COPY --link --from=build /out /out

ENTRYPOINT [ "/out" ]
