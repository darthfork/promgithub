FROM golang:1.23.10 AS builder

ENV GOOS=linux

WORKDIR /app

COPY . .

RUN make mod

RUN make test

RUN make build

FROM debian:bookworm-slim

ENV DEBIAN_FRONTEND=noninteractive

RUN groupadd -r promgithub &&\
    useradd -md /bin/bash --no-log-init -r -g promgithub promgithub

RUN apt-get update && apt-get install -y ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /app/build/promgithub .

USER promgithub

CMD ["/app/promgithub"]
