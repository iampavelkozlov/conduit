#!/bin/sh
set -eu

cert_dir=/etc/nginx/certs
public_host=${PUBLIC_HOST:-localhost}

case "$public_host" in
    *[!0-9.]* ) subject_alt_name="DNS:$public_host" ;;
    * ) subject_alt_name="IP:$public_host" ;;
esac

mkdir -p "$cert_dir"
umask 077

if [ ! -s "$cert_dir/ca.key" ] || [ ! -s "$cert_dir/ca.crt" ]; then
    echo "Generating Conduit local certificate authority"
    openssl genrsa -out "$cert_dir/ca.key" 4096
    openssl req -x509 -new -key "$cert_dir/ca.key" \
        -sha256 -days 3650 \
        -subj "/CN=Conduit Local CA" \
        -addext "basicConstraints=critical,CA:TRUE" \
        -addext "keyUsage=critical,keyCertSign,cRLSign" \
        -out "$cert_dir/ca.crt"

    rm -f "$cert_dir/server.key" "$cert_dir/server.crt" "$cert_dir/ca.srl"
fi

if [ ! -s "$cert_dir/server.key" ] || \
   [ ! -s "$cert_dir/server.crt" ] || \
   ! openssl x509 -checkend 2592000 -noout -in "$cert_dir/server.crt" >/dev/null 2>&1; then
    echo "Generating HTTPS certificate for $public_host"
    openssl genrsa -out "$cert_dir/server.key" 2048
    openssl req -new -key "$cert_dir/server.key" \
        -subj "/CN=$public_host" \
        -addext "subjectAltName=$subject_alt_name" \
        -addext "basicConstraints=critical,CA:FALSE" \
        -addext "keyUsage=critical,digitalSignature,keyEncipherment" \
        -addext "extendedKeyUsage=serverAuth" \
        -out "$cert_dir/server.csr"
    openssl x509 -req -in "$cert_dir/server.csr" \
        -CA "$cert_dir/ca.crt" \
        -CAkey "$cert_dir/ca.key" \
        -CAcreateserial \
        -copy_extensions copy \
        -sha256 -days 825 \
        -out "$cert_dir/server.crt"
    rm -f "$cert_dir/server.csr"
fi

chmod 600 "$cert_dir/ca.key" "$cert_dir/server.key"
chmod 644 "$cert_dir/ca.crt" "$cert_dir/server.crt"
