FROM dhi.io/golang:1.26.4-debian13-dev@sha256:9a675e23c51e343854caccdcf56c40a9f39b5c3024ca02ae045b48786d0d7a42 AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/chart-version-guard ./cmd/chart-version-guard

FROM dhi.io/golang:1.26.4-debian13-dev@sha256:9a675e23c51e343854caccdcf56c40a9f39b5c3024ca02ae045b48786d0d7a42

COPY --from=build /out/chart-version-guard /usr/local/bin/chart-version-guard
