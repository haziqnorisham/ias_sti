FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --chmod=0755 ias_sti .

EXPOSE 8080

CMD ["/app/ias_sti"]
