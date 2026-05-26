#!/bin/bash
# Usage: ./upload_r2.sh <file_path>

if [ -z "$1" ]; then
  echo "Error: No file path provided."
  exit 1
fi

FILE_PATH="$1"
FILENAME=$(basename "$FILE_PATH")
# Generate a random prefix to prevent overwriting
RANDOM_PREFIX=$(cat /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w 8 | head -n 1)
OBJECT_KEY="images/${RANDOM_PREFIX}_${FILENAME}"

# Credentials should be set in environment
if [ -z "$R2_ACCOUNT_ID" ] || [ -z "$R2_ACCESS_KEY_ID" ] || [ -z "$R2_SECRET_ACCESS_KEY" ] || [ -z "$R2_BUCKET_NAME" ] || [ -z "$R2_PUBLIC_DOMAIN" ]; then
  echo "Error: Missing R2 environment variables."
  exit 1
fi

# We use aws-cli to upload to R2
AWS_ACCESS_KEY_ID=$R2_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY=$R2_SECRET_ACCESS_KEY aws s3 cp "$FILE_PATH" "s3://$R2_BUCKET_NAME/$OBJECT_KEY" \
  --endpoint-url "https://$R2_ACCOUNT_ID.r2.cloudflarestorage.com" \
  --region auto \
  > /dev/null

if [ $? -eq 0 ]; then
  echo "${R2_PUBLIC_DOMAIN}/${OBJECT_KEY}"
else
  echo "Error: Upload failed."
  exit 1
fi
