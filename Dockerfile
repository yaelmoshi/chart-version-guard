FROM dhi.io/golang:1.26.3-debian13-dev@sha256:972b3b41ae70170229bd24c019dd60465130be297739a9bb64d834c21ac1a27b AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/chart-version-guard ./cmd/chart-version-guard

FROM dhi.io/golang:1.26.3-debian13-dev@sha256:972b3b41ae70170229bd24c019dd60465130be297739a9bb64d834c21ac1a27b

COPY --from=build /out/chart-version-guard /usr/local/bin/chart-version-guard
