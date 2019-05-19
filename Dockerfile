FROM golang:alpine as builder

RUN mkdir /build
ADD . /build/
WORKDIR /build
RUN go build -o seed-finder .

FROM frolvlad/alpine-glibc
COPY --from=builder /build/seed-finder /seed-finder/

RUN mkdir /factorio
ADD ./factorio /factorio/

RUN mkdir /factoriotest
COPY ./settings/gen-settings.json /seed-finder/settings/gen-settings.json
COPY ./config.ini.template /seed-finder/config.ini.template

WORKDIR /seed-finder
VOLUME /output
CMD ["./seed-finder"]