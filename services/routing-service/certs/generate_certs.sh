


#!/bin/bash

# Generate CA key and certificate
openssl genrsa -out ca.key 2048
openssl req -x509 -new -nodes -key ca.key -sha256 -days 1825 -out ca.crt -subj "/CN=Routing CA"

# Generate server key and certificate
openssl genrsa -out server.key 2048
openssl req -new -key server.key -out server.csr -config server.cnf
openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out server.crt -days 365 -sha256 -extfile server.cnf -extensions req_ext

# Generate client key and certificate
openssl genrsa -out client.key 2048
openssl req -new -key client.key -out client.csr -config client.cnf
openssl x509 -req -in client.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out client.crt -days 365 -sha256 -extfile client.cnf -extensions req_ext

echo "Certificates generated successfully!"


