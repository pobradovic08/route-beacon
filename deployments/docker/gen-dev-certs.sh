#!/usr/bin/env bash
#
# gen-dev-certs.sh -- Generate self-signed mTLS certificates for local development.
#
# Creates a CA, a server certificate (for central), and a client certificate
# (for collector). All output goes to the directory specified by CERT_DIR
# (default: /certs).
#
# The script is idempotent: if ca.pem already exists and is not expired, it
# exits early so that docker-compose restarts do not regenerate certificates
# on every run.
#
set -euo pipefail

CERT_DIR="${CERT_DIR:-/certs}"
DAYS="${CERT_DAYS:-3650}"

mkdir -p "${CERT_DIR}"

# If the CA cert already exists and is valid, skip generation.
if [ -f "${CERT_DIR}/ca.pem" ]; then
    if openssl x509 -checkend 86400 -noout -in "${CERT_DIR}/ca.pem" 2>/dev/null; then
        echo "gen-dev-certs: certificates already exist and CA is valid, skipping."
        exit 0
    fi
    echo "gen-dev-certs: CA certificate expired or invalid, regenerating all certs."
fi

echo "gen-dev-certs: generating development mTLS certificates in ${CERT_DIR}"

# ---- CA ----
openssl genrsa -out "${CERT_DIR}/ca-key.pem" 4096 2>/dev/null
openssl req -new -x509 \
    -key "${CERT_DIR}/ca-key.pem" \
    -out "${CERT_DIR}/ca.pem" \
    -days "${DAYS}" \
    -subj "/CN=route-beacon-dev-ca/O=route-beacon/OU=dev" \
    -batch 2>/dev/null

# ---- Server certificate (central) ----
openssl genrsa -out "${CERT_DIR}/central-key.pem" 2048 2>/dev/null

# Create a config file with SANs for the server cert.
cat > "${CERT_DIR}/_central.cnf" <<EOF
[req]
distinguished_name = req_dn
req_extensions = v3_req
prompt = no

[req_dn]
CN = central
O  = route-beacon
OU = dev

[v3_req]
subjectAltName = @alt_names

[alt_names]
DNS.1 = central
DNS.2 = localhost
IP.1  = 127.0.0.1
EOF

openssl req -new \
    -key "${CERT_DIR}/central-key.pem" \
    -out "${CERT_DIR}/central.csr" \
    -config "${CERT_DIR}/_central.cnf" 2>/dev/null

openssl x509 -req \
    -in "${CERT_DIR}/central.csr" \
    -CA "${CERT_DIR}/ca.pem" \
    -CAkey "${CERT_DIR}/ca-key.pem" \
    -CAcreateserial \
    -out "${CERT_DIR}/central.pem" \
    -days "${DAYS}" \
    -extensions v3_req \
    -extfile "${CERT_DIR}/_central.cnf" 2>/dev/null

# ---- Client certificate (collector) ----
# CN is set to the collector ID so that central can extract it from the
# peer certificate (see tlsutil.ExtractCollectorID).
openssl genrsa -out "${CERT_DIR}/collector-key.pem" 2048 2>/dev/null

cat > "${CERT_DIR}/_collector.cnf" <<EOF
[req]
distinguished_name = req_dn
req_extensions = v3_req
prompt = no

[req_dn]
CN = dev-collector-01
O  = route-beacon
OU = dev

[v3_req]
subjectAltName = @alt_names

[alt_names]
DNS.1 = collector
DNS.2 = localhost
IP.1  = 127.0.0.1
EOF

openssl req -new \
    -key "${CERT_DIR}/collector-key.pem" \
    -out "${CERT_DIR}/collector.csr" \
    -config "${CERT_DIR}/_collector.cnf" 2>/dev/null

openssl x509 -req \
    -in "${CERT_DIR}/collector.csr" \
    -CA "${CERT_DIR}/ca.pem" \
    -CAkey "${CERT_DIR}/ca-key.pem" \
    -CAcreateserial \
    -out "${CERT_DIR}/collector.pem" \
    -days "${DAYS}" \
    -extensions v3_req \
    -extfile "${CERT_DIR}/_collector.cnf" 2>/dev/null

# Clean up intermediate files
rm -f "${CERT_DIR}"/*.csr "${CERT_DIR}"/_*.cnf "${CERT_DIR}"/*.srl

# Set permissions so containers can read them
chmod 644 "${CERT_DIR}"/*.pem
chmod 600 "${CERT_DIR}"/*-key.pem

echo "gen-dev-certs: done. Files in ${CERT_DIR}:"
ls -la "${CERT_DIR}"/*.pem
