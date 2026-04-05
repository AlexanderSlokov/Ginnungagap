#!/bin/bash
# -----------------------------------------------------------------------------
# Ginnungagap - Dynamic Honeypot Credential Generator
# -----------------------------------------------------------------------------
# This script runs dynamically inside the Sandbox at runtime to generate
# fake credentials (AWS, Kubeconfig, SSH, GCP, NPM, Terraform) dynamically.
# Purpose: Deceive malware and prevent script kiddies from fingerprinting
# static honey tokens by randomizing them on every container start.
# -----------------------------------------------------------------------------

set -e

USER_HOME="/home/${USER_NAME:-dev}"
mkdir -p "${USER_HOME}"

generate_random_string() {
    local length=$1
    # Use awk/tr instead of head to read from /dev/urandom to avoid the "invalid number of bytes" error in some distros
    tr -dc 'a-zA-Z0-9' < /dev/urandom | head -c "${length}"
}

# Function to generate random Base64 string
generate_random_base64() {
    local length=$1
    head -c "${length}" /dev/urandom | base64 | tr -d '\n' | cut -c 1-"${length}"
}

echo "Ginnungagap: Planting dynamic honeypots at runtime..."

# =============================================================================
# 1. AWS Credentials (Fake)
# =============================================================================
AWS_DIR="${USER_HOME}/.aws"
mkdir -p "${AWS_DIR}"

FAKE_AWS_ACCESS_KEY_ID="AKIA$(generate_random_string 16 | tr 'a-z' 'A-Z')"
FAKE_AWS_SECRET_ACCESS_KEY="$(generate_random_base64 40)"

cat <<EOF > "${AWS_DIR}/credentials"
[default]
aws_access_key_id = ${FAKE_AWS_ACCESS_KEY_ID}
aws_secret_access_key = ${FAKE_AWS_SECRET_ACCESS_KEY}
EOF

cat <<EOF > "${AWS_DIR}/config"
[default]
region = us-west-2
output = json
EOF

chmod 600 "${AWS_DIR}/credentials" "${AWS_DIR}/config"
echo "  - Planted: AWS Credentials"

# =============================================================================
# 2. Kubernetes Config (Fake Kubeconfig)
# =============================================================================
KUBE_DIR="${USER_HOME}/.kube"
mkdir -p "${KUBE_DIR}"

FAKE_KUBE_CLUSTER_NAME="cluster-$(generate_random_string 6 | tr 'A-Z' 'a-z')"
FAKE_KUBE_CONTEXT="ctx-${FAKE_KUBE_CLUSTER_NAME}"
FAKE_KUBE_TOKEN="ey$(generate_random_base64 36).$(generate_random_base64 40)"
FAKE_KUBE_SERVER_IP="10.$(($RANDOM % 255)).$(($RANDOM % 255)).$(($RANDOM % 255))"

cat <<EOF > "${KUBE_DIR}/config"
apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: $(generate_random_base64 64)
    server: https://${FAKE_KUBE_SERVER_IP}:6443
  name: ${FAKE_KUBE_CLUSTER_NAME}
contexts:
- context:
    cluster: ${FAKE_KUBE_CLUSTER_NAME}
    user: admin-${FAKE_KUBE_CLUSTER_NAME}
  name: ${FAKE_KUBE_CONTEXT}
current-context: ${FAKE_KUBE_CONTEXT}
kind: Config
preferences: {}
users:
- name: admin-${FAKE_KUBE_CLUSTER_NAME}
  user:
    token: ${FAKE_KUBE_TOKEN}
EOF

chmod 600 "${KUBE_DIR}/config"
echo "  - Planted: Kubernetes Config"

# =============================================================================
# 3. SSH Keys (Using ssh-keygen with random algorithms)
# =============================================================================
SSH_DIR="${USER_HOME}/.ssh"
mkdir -p "${SSH_DIR}"

ALGORITHMS=("rsa" "ed25519" "ecdsa")
# Generate a random number of keys between 1 and 4
NUM_KEYS=$((1 + RANDOM % 4))

for ((i=1; i<=NUM_KEYS; i++)); do
    ALGO=${ALGORITHMS[$RANDOM % ${#ALGORITHMS[@]}]}
    KEY_NAME="id_${ALGO}_${i}"
    # Generate the key quietly, with no passphrase
    ssh-keygen -t "${ALGO}" -N "" -f "${SSH_DIR}/${KEY_NAME}" -q
done

# Try to symlink the first generated key to standard id_rsa or id_ed25519 name
# so naive malwares looking for default names can find it easily
FIRST_KEY=$(ls "${SSH_DIR}" | grep -v '\.pub$' | head -n 1)
if [[ -n "${FIRST_KEY}" && "${FIRST_KEY}" != id_rsa && "${FIRST_KEY}" != id_ed25519 ]]; then
    ln -s "${SSH_DIR}/${FIRST_KEY}" "${SSH_DIR}/id_rsa"
fi

# Add known_hosts for github.com so clones don't prompt
echo "github.com ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQCj7ndNxQowgcQnjshz..." > "${SSH_DIR}/known_hosts"

chmod 700 "${SSH_DIR}"
chmod 600 "${SSH_DIR}"/* || true
echo "  - Planted: ${NUM_KEYS} random SSH Keys"

# =============================================================================
# 4. GCP Application Default Credentials
# =============================================================================
GCP_DIR="${USER_HOME}/.config/gcloud"
mkdir -p "${GCP_DIR}"

cat <<EOF > "${GCP_DIR}/application_default_credentials.json"
{
  "client_id": "$(generate_random_string 24).apps.googleusercontent.com",
  "client_secret": "$(generate_random_string 30)",
  "refresh_token": "1//$(generate_random_string 60)",
  "type": "authorized_user"
}
EOF
chmod 600 "${GCP_DIR}/application_default_credentials.json"
echo "  - Planted: GCP Application Default Credentials"

# =============================================================================
# 5. Terraform State file (.tfstate)
# =============================================================================
APP_DIR="${USER_HOME}/app"
mkdir -p "${APP_DIR}"

cat <<EOF > "${APP_DIR}/terraform.tfstate"
{
  "version": 4,
  "terraform_version": "1.5.0",
  "serial": 3,
  "lineage": "$(generate_random_string 8)-$(generate_random_string 4)-$(generate_random_string 4)-$(generate_random_string 4)-$(generate_random_string 12)",
  "outputs": {
    "db_password": {
      "value": "super_secret_$(generate_random_string 12)",
      "type": "string",
      "sensitive": true
    }
  },
  "resources": []
}
EOF
echo "  - Planted: Terraform State file"

# =============================================================================
# 6. Local .env file with secrets
# =============================================================================
cat <<EOF > "${APP_DIR}/.env"
GEMINI_API_KEY="AIzaSy$(generate_random_string 33)"
STRIPE_SECRET_KEY="sk_live_$(generate_random_string 24)"
NPM_TOKEN="npm_$(generate_random_string 36)"
DATABASE_URL="postgres://admin:$(generate_random_string 16)@db.internal.local:5432/production"
EOF
echo "  - Planted: .env file with secrets"

# =============================================================================
# 7. Git Config & Bash History (Fake footprint)
# =============================================================================
cat <<EOF > "${USER_HOME}/.gitconfig"
[user]
	name = ${USER_NAME:-dev}
	email = ${USER_NAME:-dev}@company.internal.local
[credential]
	helper = store
EOF

cat <<EOF > "${USER_HOME}/.bash_history"
cd ~/app
npm install
npm run build
git status
git pull origin main
docker ps
kubectl get pods -n production
aws s3 ls
vim package.json
EOF
chmod 600 "${USER_HOME}/.bash_history"

echo "Ginnungagap: Sandbox is fully armed and operational."

# =============================================================================
# Execute the original container CMD (This makes it a valid Entrypoint script)
# =============================================================================
exec "$@"