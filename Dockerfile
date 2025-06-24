FROM golang:1.24-bookworm AS build

WORKDIR /go/src/project/
COPY . /go/src/project/

RUN make release
RUN mkdir "/data" && \
    echo "" > /config.toml

FROM scratch
WORKDIR /
COPY --from=build /go/src/project/bin/goFSL /bin/goFSL
COPY --from=build /data /data
COPY --from=build /config.toml /config.toml


EXPOSE 8080:8080
LABEL description="goFSL"
ENTRYPOINT ["/bin/goFSL", "-dataDir",  "/data", "-configFile", "/data/config.toml"]