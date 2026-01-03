FROM golang:1.25 as builder

ENV GOPATH=/root/go
RUN mkdir -p /root/go/src
COPY dyndns /root/go/src/dyndns
WORKDIR /root/go/src/dyndns
RUN go mod tidy
RUN GOOS=linux go build -o /root/go/bin/dyndns && go test -v

FROM debian:13-slim

RUN DEBIAN_FRONTEND=noninteractive apt-get update && \
	apt-get install -q -y bind9 dnsutils curl && \
	apt-get clean

RUN chmod 770 /var/cache/bind
COPY deployment/setup.sh /root/setup.sh 
RUN chmod +x /root/setup.sh
COPY deployment/entrypoint.sh /root/entrypoint.sh
RUN chmod +x /root/entrypoint.sh
COPY deployment/named.conf.options /etc/bind/named.conf.options

WORKDIR /root
COPY --from=builder /root/go/bin/dyndns /root/dyndns
COPY dyndns/views /root/views
COPY dyndns/static /root/static

EXPOSE 53 8080
CMD ["/root/entrypoint.sh"]
