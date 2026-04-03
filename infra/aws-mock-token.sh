#!/usr/bin/env bash
# Mock for 'aws eks get-token' in LocalStack test environment
# Returns a static token since LocalStack doesn't implement real EKS auth
cat <<EOF
{
  "kind": "ExecCredential",
  "apiVersion": "client.authentication.k8s.io/v1beta1",
  "status": {
    "token": "test-token"
  }
}
EOF
