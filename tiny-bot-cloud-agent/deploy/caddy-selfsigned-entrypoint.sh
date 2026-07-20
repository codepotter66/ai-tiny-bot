#!/bin/sh
set -e

CERT_DIR=/etc/caddy/certs
mkdir -p "$CERT_DIR"

IP="${TB_PUBLIC_IP:-}"
if [ -z "$IP" ]; then
	echo "❌ TB_PUBLIC_IP 未设置（自签 HTTPS 需要证书 SAN IP）"
	echo "   请在 .env 中设置：TB_PUBLIC_IP=<服务器公网IP>"
	exit 1
fi

if [ ! -f "$CERT_DIR/cert.pem" ] || [ ! -f "$CERT_DIR/key.pem" ]; then
	echo "生成自签 TLS 证书（IP=$IP）..."
	openssl req -x509 -nodes -days 825 -newkey rsa:2048 \
		-keyout "$CERT_DIR/key.pem" \
		-out "$CERT_DIR/cert.pem" \
		-subj "/CN=$IP" \
		-addext "subjectAltName=IP:$IP,DNS:localhost" 2>/dev/null \
		|| openssl req -x509 -nodes -days 825 -newkey rsa:2048 \
			-keyout "$CERT_DIR/key.pem" \
			-out "$CERT_DIR/cert.pem" \
			-subj "/CN=$IP"
fi

exec caddy run --config /etc/caddy/Caddyfile --adapter caddyfile
