FROM golang:1.10

RUN mkdir -p /go/src/github.com/Lambda-NIC/faas-netes/

WORKDIR /go/src/github.com/Lambda-NIC/faas-netes

COPY . .

RUN gofmt -l -d $(find . -type f -name '*.go' -not -path "./vendor/*") \
    && go test ./test/ \
    && VERSION=$(git describe --all --exact-match `git rev-parse HEAD` | grep tags | sed 's/tags\///') \
    && GIT_COMMIT=$(git rev-list -1 HEAD) \
    && CGO_ENABLED=0 GOOS=linux go build --ldflags "-s -w \
        -X github.com/Lambda-NIC/faas-netes/version.GitCommit=${GIT_COMMIT}\
        -X github.com/Lambda-NIC/faas-netes/version.Version=${VERSION}" \
        -a -installsuffix cgo -o faas-netes .

FROM alpine:3.8

LABEL org.label-schema.license="MIT" \
      org.label-schema.vcs-url="https://github.com/Lambda-NIC/faas-netes" \
      org.label-schema.vcs-type="Git" \
      org.label-schema.name="Lambda-NIC/faas-netes" \
      org.label-schema.vendor="Lambda-NIC" \
      org.label-schema.docker.schema-version="1.0"

RUN addgroup -S app \
    && adduser -S -g app app \
    && apk --no-cache add \
    ca-certificates
WORKDIR /home/app

EXPOSE 8080

ENV http_proxy      ""
ENV https_proxy     ""

COPY --from=0 /go/src/github.com/Lambda-NIC/faas-netes/faas-netes    .
RUN chown -R app:app ./

USER app

CMD ["./faas-netes"]
