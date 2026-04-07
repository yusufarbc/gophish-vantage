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


# Runtime container with ProjectDiscovery tools
FROM debian:bookworm-slim

RUN useradd -m -d /opt/gophish -s /bin/bash app

# Install dependencies and ProjectDiscovery tools
RUN apt-get update && \
	apt-get install --no-install-recommends -y \
		jq libcap2-bin ca-certificates \
		wget git curl unzip \
		golang-go git \
		libpcap-dev \
		net-tools iputils-ping dnsutils && \
	apt-get clean && \
	rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

# Install ProjectDiscovery tools using the pdtm (ProjectDiscovery Tool Manager) or manual installation
RUN mkdir -p /opt/pd-tools && \
	cd /opt/pd-tools && \
	go install -v github.com/projectdiscovery/subfinder/v2/cmd/subfinder@latest && \
	go install -v github.com/projectdiscovery/nuclei/v2/cmd/nuclei@latest && \
	go install -v github.com/projectdiscovery/httpx/cmd/httpx@latest && \
	go install -v github.com/projectdiscovery/naabu/v2/cmd/naabu@latest && \
	go install -v github.com/projectdiscovery/dnsx/cmd/dnsx@latest && \
	go install -v github.com/projectdiscovery/katana/cmd/katana@latest && \
	go install -v github.com/projectdiscovery/tlsx/cmd/tlsx@latest && \
	go install -v github.com/projectdiscovery/assetfinder@latest && \
	go install -v github.com/projectdiscovery/asnmap/cmd/asnmap@latest && \
	go install -v github.com/projectdiscovery/uncover/cmd/uncover@latest && \
	# Chisel — reverse TUN tunnel server/client
	go install -v github.com/jpillora/chisel@latest

# Ensure tools are in PATH
ENV PATH="/root/go/bin:${PATH}"

WORKDIR /opt/gophish
COPY --from=build-golang /go/src/github.com/gophish/gophish/ ./
COPY --from=build-js /build/static/js/dist/ ./static/js/dist/
COPY --from=build-js /build/static/css/dist/ ./static/css/dist/
COPY --from=build-golang /go/src/github.com/gophish/gophish/config.json ./
RUN chown app. config.json

RUN setcap 'cap_net_bind_service=+ep' /opt/gophish/gophish

USER app
RUN sed -i 's/127.0.0.1/0.0.0.0/g' config.json
RUN touch config.json.tmp

EXPOSE 3333 8080 8443 80 9090

CMD ["./docker/run.sh"]
