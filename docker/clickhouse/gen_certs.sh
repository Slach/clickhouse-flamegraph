#!/bin/bash

CUR_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"


mkdir -p -m 0755 ${CUR_DIR}/ssl

if [[!-f ${CUR_DIR}/ssl/dhparam.pem ]]; then
  openssl dhparam -out ${CUR_DIR}/ssl/dhparam.pem 4096
fi

cat <<EOF > ${CUR_DIR}/ssl/ssl.conf

[req]
prompt = no
distinguished_name  = subject
req_extensions      = req_ext
x509_extensions     = x509_ext
string_mask         = utf8only

[subject]
CN = ClickHouse

[x509_ext]
subjectKeyIdentifier   = hash
authorityKeyIdentifier = keyid,issuer

basicConstraints  = CA:FALSE
keyUsage          = digitalSignature, keyEncipherment
subjectAltName    = @alternate_names
nsComment         = "OpenSSL Generated Certificate"

[req_ext]
subjectKeyIdentifier = hash
basicConstraints     = CA:FALSE
keyUsage             = digitalSignature, keyEncipherment
subjectAltName       = @alternate_names
nsComment            = "OpenSSL Generated Certificate"

[alternate_names]
DNS.1       = localhost
DNS.2       = localhost.localdomain
DNS.3       = 127.0.0.1

[root_ca]
subjectKeyIdentifier = hash
authorityKeyIdentifier = keyid:always,issuer
basicConstraints = critical, CA:true
keyUsage = critical, digitalSignature, cRLSign, keyCertSign
EOF


openssl req -x509 -newkey rsa:2048 -config "${CUR_DIR}/ssl/ssl.conf" -extensions root_ca -keyout "${CUR_DIR}/ssl/ca-key.pem" -nodes -out "${CUR_DIR}/ssl/ca-cert.pem"
openssl req -config "${CUR_DIR}/ssl/ssl.conf" -new -x509 -sha256 -newkey rsa:2048 -nodes -days 365 -keyout "${CUR_DIR}/ssl/clickhouse-key.pem" -out "${CUR_DIR}/ssl/clickhouse-cert.pem"
openssl req -new -key "${CUR_DIR}/ssl/clickhouse-key.pem" -config "${CUR_DIR}/ssl/ssl.conf" -reqexts req_ext -out "${CUR_DIR}/ssl/clickhouse.csr"
openssl x509 -req -days 365 -CA "${CUR_DIR}/ssl/ca-cert.pem" -CAkey "${CUR_DIR}/ssl/ca-key.pem" -set_serial 01 -extfile "${CUR_DIR}/ssl/ssl.conf" -extensions req_ext -in "${CUR_DIR}/ssl/clickhouse.csr" -out "${CUR_DIR}/ssl/clickhouse.crt"
openssl verify -CAfile "${CUR_DIR}/ssl/ca-cert.pem" "${CUR_DIR}/ssl/clickhouse.crt"

docker-compose down
docker-compose up -d clickhouse
curl -vvv --cacert "${CUR_DIR}/ssl/ca-cert.pem" --key "${CUR_DIR}/ssl/clickhouse-key.pem" --cert "${CUR_DIR}/ssl/clickhouse.crt" "https://localhost:8443/ping"
