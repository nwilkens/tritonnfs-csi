#\!/bin/bash
set -e

# Set variables from Kubernetes secrets
ACCOUNT_ID=$(kubectl get secret -n kube-system triton-creds -o jsonpath='{.data.account-id}' | base64 -d)
KEY_ID=$(kubectl get secret -n kube-system triton-creds -o jsonpath='{.data.key-id}' | base64 -d)
CLOUDAPI=$(kubectl get secret -n kube-system triton-creds -o jsonpath='{.data.cloudapi}' | base64 -d)

# Create a temporary file for the private key
KEY_FILE=$(mktemp)
kubectl get secret -n kube-system triton-creds -o jsonpath='{.data.key\.pem}' | base64 -d > $KEY_FILE
chmod 600 $KEY_FILE

# Print key information
echo "Private key file: $KEY_FILE"
echo "Key contents (first few lines):"
head -n 3 $KEY_FILE

# Format variables as needed for the request
ALG="rsa-sha256"
KEY_ID_FORMAT="/$ACCOUNT_ID/keys/$KEY_ID"
NOW=$(date -u "+%a, %d %h %Y %H:%M:%S GMT")

# For debugging
echo "Signing string: date: $NOW"

# Generate the signature
SIG=$(echo -n "date: $NOW" | openssl dgst -sha256 -sign $KEY_FILE | base64)

# Make the API request
echo "Making API request with:"
echo "Account ID: $ACCOUNT_ID"
echo "Key ID: $KEY_ID"
echo "Formatted Key ID: $KEY_ID_FORMAT"
echo "CloudAPI: $CLOUDAPI"
echo "Date: $NOW"
echo "Signature (first 20 chars): ${SIG:0:20}..."
echo

# Construct the Authorization header
AUTH_HEADER="Signature keyId=\"$KEY_ID_FORMAT\",algorithm=\"$ALG\",headers=\"date\",signature=\"$SIG\""

# Debug output
echo "Full Authorization header: $AUTH_HEADER"

# Perform the API call
RESPONSE=$(curl -sS "$CLOUDAPI/my/volumes" \
     -H "date: $NOW" \
     -H "Authorization: $AUTH_HEADER" \
     -w "\nHTTP_STATUS:%{http_code}")

# Extract HTTP status code
HTTP_STATUS=$(echo "$RESPONSE" | grep "HTTP_STATUS:" | cut -d":" -f2)
RESPONSE_BODY=$(echo "$RESPONSE" | grep -v "HTTP_STATUS:")

# Output the results
echo "Response HTTP Status: $HTTP_STATUS"
echo "Response Body: $RESPONSE_BODY"

# Clean up the key file
rm -f $KEY_FILE
