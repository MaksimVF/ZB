#!/bin/bash
set -e

mkdir -p certs
cd certs

# Установка cfssl, если нет
if ! command -f cfssl; then
  curl -L https://github.com/cloudflare/cfssl/releases/download/v1.6.4/cfssl_1.6.4_linux_amd64 -o cfssl
  curl -L https://github.com/cloudflare/cfssl/releases/download/v1.6.4/cfssljson_1.6.4_linux_amd64 -o cfssljson
  chmod +x cfssl cfssljson
fi

# CA
cat > ca-config.json <<EOF
{
  "signing": {
    "default": { "expiry": "87600h" },
    "profiles": { "server": { "expiry": "87600h", "usages": ["signing","key encipherment","server auth"] },
                  "client": { "expiry": "87600h", "usages": ["signing","key encipherment","client auth"] }}
  }
}
EOF

cat > ca-csr.json <<EOF
{ "CN": "LLM Platform Root CA", "key": { "algo": "rsa", "size": 4096 }, "names": [{ "C": "RU", "O": "MyCompany" }] }
EOF

./cfssl gencert -initca ca-csr.json | ./cfssljson -bare ca

# Сертификаты для сервисов
for service in head model-proxy; do
  cat > ${service}-csr.json <<EOF
{
  "CN": "${service}",
  "key": { "algo": "rsa", "size": 2048 },
  "names": [{ "C": "RU", "O": "MyCompany" }],
  "hosts": ["localhost", "127.0.0.1", "${service}", "${service}.localhost"]
}
EOF
  ./cfssl gencert -ca=ca.pem -ca-key=ca-key.pem -config=ca-config.json -profile=server ${service}-csr.json | ./cfssljson -bare ${service}
done

echo "mTLS сертификаты сгенерированы в ./certs/"
