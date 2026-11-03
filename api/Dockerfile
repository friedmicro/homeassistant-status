FROM docker.friedmicro-lab.org/base AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY main.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/homeassistantstatus .

FROM docker.friedmicro-lab.org/base-slim
COPY --from=build /out/homeassistantstatus /usr/local/bin/homeassistantstatus
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/homeassistantstatus"]
