FROM dhi.io/golang:1.26.4-debian13-dev@sha256:9350c6e0c0a06f23b487c39850188b2a5c2bd176d1fc5ae361400c8c068e42b0 AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/chart-version-guard ./cmd/chart-version-guard

FROM dhi.io/golang:1.26.4-debian13-dev@sha256:9350c6e0c0a06f23b487c39850188b2a5c2bd176d1fc5ae361400c8c068e42b0

COPY --from=build /out/chart-version-guard /usr/local/bin/chart-version-guard
