FROM dhi.io/golang:1.26.3-debian13-dev@sha256:fda946333293b2f23e6cf3486403c92590483367c272ffc7d2c12362342e1389 AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/chart-version-guard ./cmd/chart-version-guard

FROM dhi.io/golang:1.26.3-debian13-dev@sha256:fda946333293b2f23e6cf3486403c92590483367c272ffc7d2c12362342e1389

COPY --from=build /out/chart-version-guard /usr/local/bin/chart-version-guard
