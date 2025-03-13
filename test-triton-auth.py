#\!/usr/bin/env python3
import os
import sys
import base64
import json
import subprocess
import tempfile
import urllib.request
import datetime
import re

# Get the credentials from Kubernetes secrets
account_id = subprocess.check_output(
    "kubectl get secret -n kube-system triton-creds -o jsonpath='{.data.account-id}' | base64 -d",
    shell=True
).decode('utf-8').strip()

key_id = subprocess.check_output(
    "kubectl get secret -n kube-system triton-creds -o jsonpath='{.data.key-id}' | base64 -d",
    shell=True
).decode('utf-8').strip()

cloudapi = subprocess.check_output(
    "kubectl get secret -n kube-system triton-creds -o jsonpath='{.data.cloudapi}' | base64 -d",
    shell=True
).decode('utf-8').strip()

# Create a temporary file for the private key
with tempfile.NamedTemporaryFile(delete=False) as key_file:
    key_path = key_file.name
    key_data = subprocess.check_output(
        "kubectl get secret -n kube-system triton-creds -o jsonpath='{.data.key\\.pem}' | base64 -d",
        shell=True
    )
    key_file.write(key_data)

try:
    print(f"Private key file: {key_path}")
    with open(key_path, 'r') as f:
        print("Key type:", f.readline().strip())
    
    # Format variables as needed for the request
    alg = "rsa-sha256"
    key_id_format = f"/{account_id}/keys/{key_id}"
    now = datetime.datetime.utcnow().strftime("%a, %d %b %Y %H:%M:%S GMT")
    
    print(f"Signing string: date: {now}")
    
    # Generate signature using ssh-keygen for ssh keys
    with tempfile.NamedTemporaryFile(delete=False) as signing_data:
        signing_path = signing_data.name
        signing_data.write(f"date: {now}".encode('utf-8'))
    
    try:
        # Try using ssh-keygen to sign the data
        signature = subprocess.check_output(
            f"ssh-keygen -Y sign -f {key_path} -n application {signing_path}",
            shell=True
        ).decode('utf-8')
        
        # Extract the signature from the output
        matches = re.search(r'-----BEGIN SSH SIGNATURE-----\n(.*?)\n-----END SSH SIGNATURE-----', signature, re.DOTALL)
        if matches:
            signature_b64 = matches.group(1).replace('\n', '')
        else:
            print("Failed to extract signature")
            sys.exit(1)
    finally:
        os.unlink(signing_path)
    
    # Make the API request
    print("Making API request with:")
    print(f"Account ID: {account_id}")
    print(f"Key ID: {key_id}")
    print(f"Formatted Key ID: {key_id_format}")
    print(f"CloudAPI: {cloudapi}")
    print(f"Date: {now}")
    print(f"Signature (first 20 chars): {signature_b64[:20]}...")
    
    # Construct the Authorization header
    auth_header = f'Signature keyId="{key_id_format}",algorithm="{alg}",headers="date",signature="{signature_b64}"'
    
    # Debug output
    print(f"Full Authorization header length: {len(auth_header)}")
    
    # Create the request
    req = urllib.request.Request(f"{cloudapi}/my/volumes")
    req.add_header("date", now)
    req.add_header("Authorization", auth_header)
    
    try:
        # Perform the request
        with urllib.request.urlopen(req) as response:
            response_body = response.read().decode('utf-8')
            print(f"Response HTTP Status: {response.getcode()}")
            print(f"Response Body: {response_body}")
    except urllib.error.HTTPError as e:
        print(f"Response HTTP Status: {e.code}")
        print(f"Response Body: {e.read().decode('utf-8')}")
    except Exception as e:
        print(f"Error: {str(e)}")
finally:
    # Clean up the key file
    if os.path.exists(key_path):
        os.unlink(key_path)
