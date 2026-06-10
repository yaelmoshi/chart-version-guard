FROM dhi.io/golang:1.26.4-debian13-dev@sha256:17e7e33c3c31ec622fe56db4e7f20babf8d4829f41a8e6657da5d06b348a5577 AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/chart-version-guard ./cmd/chart-version-guard

FROM dhi.io/golang:1.26.4-debian13-dev@sha256:17e7e33c3c31ec622fe56db4e7f20babf8d4829f41a8e6657da5d06b348a5577

COPY --from=build /out/chart-version-guard /usr/local/bin/chart-version-guard
