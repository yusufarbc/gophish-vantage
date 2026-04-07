# Minify client side assets (JavaScript)
FROM node:latest AS build-js

RUN npm install gulp gulp-cli -g

WORKDIR /build
COPY . .
RUN npm install --only=dev
RUN gulp


# Build Golang binary
FROM golang:1.20 AS build-golang

WORKDIR /go/src/github.com/gophish/gophish
COPY . .
RUN go get -v && go build -v


# Build and pin security tooling binaries
FROM golang:1.20 AS pd-tools

RUN go install -v github.com/projectdiscovery/subfinder/v2/cmd/subfinder@latest && \
	go install -v github.com/projectdiscovery/httpx/cmd/httpx@latest && \
	go install -v github.com/projectdiscovery/nuclei/v3/cmd/nuclei@latest && \
	go install -v github.com/projectdiscovery/naabu/v2/cmd/naabu@latest && \
	go install -v github.com/projectdiscovery/dnsx/cmd/dnsx@latest && \
	go install -v github.com/projectdiscovery/katana/cmd/katana@latest && \
	go install -v github.com/projectdiscovery/tlsx/cmd/tlsx@latest && \
	go install -v github.com/projectdiscovery/asnmap/cmd/asnmap@latest && \
	go install -v github.com/projectdiscovery/uncover/cmd/uncover@latest && \
	go install -v github.com/projectdiscovery/interactsh/cmd/interactsh-client@latest && \
	go install -v github.com/projectdiscovery/cloudlist/cmd/cloudlist@latest && \
	go install -v github.com/tomnomnom/assetfinder@latest && \
	go install -v github.com/rakyll/hey@latest && \
	go install -v github.com/codesenberg/bombardier@latest && \
	go install -v github.com/tsenart/vegeta@latest



# Runtime container with ProjectDiscovery tools
FROM debian:bookworm-slim

RUN useradd -m -d /opt/gophish -s /bin/bash app

ARG CHISEL_VERSION=1.10.1

# Install dependencies and ProjectDiscovery tools
RUN apt-get update && \
	apt-get install --no-install-recommends -y \
		jq libcap2-bin ca-certificates \
		wget git curl unzip \
		libpcap-dev \
		iproute2 \
		net-tools iputils-ping dnsutils && \
	curl -sSL -o /tmp/chisel.gz https://github.com/jpillora/chisel/releases/download/v${CHISEL_VERSION}/chisel_${CHISEL_VERSION}_linux_amd64.gz && \
	gunzip /tmp/chisel.gz && \
	mv /tmp/chisel /usr/local/bin/chisel && \
	chmod +x /usr/local/bin/chisel && \
	apt-get clean && \
	rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*


# NOTE: Debug mode image skips PD tool compilation to keep build stable/fast.
# Tools can be added later via dedicated tool image or pinned release binaries.

WORKDIR /opt/gophish
COPY --from=build-golang /go/src/github.com/gophish/gophish/ ./
COPY --from=build-js /build/static/js/dist/ ./static/js/dist/
COPY --from=build-js /build/static/css/dist/ ./static/css/dist/
COPY --from=build-golang /go/src/github.com/gophish/gophish/config.json ./
COPY --from=pd-tools /go/bin/subfinder /usr/local/bin/subfinder
COPY --from=pd-tools /go/bin/httpx /usr/local/bin/httpx
COPY --from=pd-tools /go/bin/nuclei /usr/local/bin/nuclei
COPY --from=pd-tools /go/bin/naabu /usr/local/bin/naabu
COPY --from=pd-tools /go/bin/dnsx /usr/local/bin/dnsx
COPY --from=pd-tools /go/bin/katana /usr/local/bin/katana
COPY --from=pd-tools /go/bin/tlsx /usr/local/bin/tlsx
COPY --from=pd-tools /go/bin/asnmap /usr/local/bin/asnmap
COPY --from=pd-tools /go/bin/uncover /usr/local/bin/uncover
COPY --from=pd-tools /go/bin/interactsh-client /usr/local/bin/interactsh-client
COPY --from=pd-tools /go/bin/cloudlist /usr/local/bin/cloudlist
COPY --from=pd-tools /go/bin/assetfinder /usr/local/bin/assetfinder
COPY --from=pd-tools /go/bin/hey /usr/local/bin/hey
COPY --from=pd-tools /go/bin/bombardier /usr/local/bin/bombardier
COPY --from=pd-tools /go/bin/vegeta /usr/local/bin/vegeta
RUN chown app. config.json
RUN sed -i 's/\r$//' ./docker/run.sh
RUN chmod +x ./docker/run.sh

RUN setcap 'cap_net_bind_service=+ep' /opt/gophish/gophish

USER app
RUN sed -i 's/127.0.0.1/0.0.0.0/g' config.json
RUN touch config.json.tmp

EXPOSE 3333 8080 8443 80 9090

CMD ["/bin/bash", "./docker/run.sh"]
