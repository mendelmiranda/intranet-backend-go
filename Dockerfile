FROM golang:1.26-alpine AS build

ARG SERVICE
WORKDIR /src
COPY . .
RUN test -n "$SERVICE" \
	&& cd "$SERVICE" \
	&& go mod download \
	&& CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/service ./cmd/api

FROM scratch
ARG SERVICE
COPY --from=build /out/service /service
USER 65532:65532
ENTRYPOINT ["/service"]
