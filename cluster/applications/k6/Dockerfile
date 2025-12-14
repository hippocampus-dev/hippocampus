# syntax=docker/dockerfile:1.4

FROM grafana/xk6:1.0.1 AS builder

RUN xk6 build v1.1.0 --with github.com/grafana/xk6-dashboard@v0.7.6 --with github.com/grafana/xk6-sql@v1.0.5

FROM debian:bookworm-slim AS debian

# sleep requires glibc
#FROM gcr.io/distroless/static:nonroot
FROM gcr.io/distroless/base:nonroot
COPY --link --from=builder /xk6/k6 /usr/local/bin/k6
COPY --link --from=debian /bin/sleep /bin/sleep

USER 65532

ENTRYPOINT ["/usr/local/bin/k6"]
