#!/bin/sh
set -e

# Default values
: ${AWS_ACCESS_KEY_ID:=tempo}
: ${AWS_SECRET_ACCESS_KEY:=supersecret}
: ${BUCKET_NAME:=tempo}
: ${S3_PORT:=9000}

echo "Starting SeaweedFS with S3 support..."

# Create S3 config with credentials
cat > /tmp/s3config.json <<EOF
{
  "identities": [
    {
      "name": "default",
      "credentials": [
        {
          "accessKey": "${AWS_ACCESS_KEY_ID}",
          "secretKey": "${AWS_SECRET_ACCESS_KEY}"
        }
      ],
      "actions": [
        "Admin",
        "Read",
        "Write"
      ]
    }
  ]
}
EOF

# Start weed server in the background with S3 support
weed server \
  -dir=/data \
  -s3 \
  -s3.port=${S3_PORT} \
  -s3.config=/tmp/s3config.json &

WEED_PID=$!

# Wait for SeaweedFS S3 API to be ready
echo "Waiting for SeaweedFS to initialize..."
for i in $(seq 1 60); do
  # Use 127.0.0.1 to force IPv4, accept any HTTP response as evidence S3 API is running
  if wget --spider http://127.0.0.1:${S3_PORT}/ 2>&1 | grep -E "HTTP|connected"; then
    echo "SeaweedFS S3 API is ready!"
    break
  fi
  if [ $i -eq 60 ]; then
    echo "Timeout waiting for SeaweedFS S3 API"
    exit 1
  fi
  sleep 1
done

# Create bucket using weed shell
echo "Creating bucket '${BUCKET_NAME}'..."
weed shell <<SHELL_EOF
s3.bucket.create -name=${BUCKET_NAME}
exit
SHELL_EOF

echo "SeaweedFS is running with bucket '${BUCKET_NAME}' created"

# Wait for the weed server process
wait $WEED_PID
