# --- build stage ---
FROM docker.io/library/golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY main.go ./
COPY static ./static
RUN go mod tidy && \
    CGO_ENABLED=0 go build -ldflags="-s -w" -o /callapp .

# --- runtime stage ---
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /callapp /callapp
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/callapp"]
