#!/bin/bash
# -----------------------------------------------------------------------------
# Ginnungagap - Dynamic Honeypot Credential Generator
# -----------------------------------------------------------------------------
# Script này được chạy ngầm trong Sandbox để tạo ra những thông tin giả mạo (AWS,
# Kubeconfig, SSH, GCP, NPM) một cách ngẫu nhiên nhưng ĐÚNG ĐỊNH DẠNG.
# Mục đích: Đánh lừa malware và tránh bị đoán được signature.
# -----------------------------------------------------------------------------

set -e # Dừng script nếu có lỗi

USER_HOME="/home/${USER_NAME:-dev}"
mkdir -p "${USER_HOME}"

# Hàm tạo chuỗi ngẫu nhiên
generate_random_string() {
    local length=$1
    head -c /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w "${length}" | head -n 1
}

# Hàm tạo Base64 ngẫu nhiên
generate_random_base64() {
    local length=$1
    head -c "${length}" /dev/urandom | base64 | tr -d '\n'
}

echo "Ginnungagap: Planting dynamic honeypots..."

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
FAKE_KUBE_TOKEN="ey$(generate_random_base64 120).$(generate_random_base64 80)"
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
# 3. SSH Keys (Fake id_rsa & id_ed25519)
# =============================================================================
SSH_DIR="${USER_HOME}/.ssh"
mkdir -p "${SSH_DIR}"

# Tạo fake RSA key (không dùng ssh-keygen để tránh tốn thời gian, chỉ cần format giống thật)
cat <<EOF > "${SSH_DIR}/id_rsa"
-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAABlwAAAAdzc2gtcn
NhAAAAAwEAAQAAAYEA...
$(generate_random_base64 800 | fold -w 70)
...
-----END OPENSSH PRIVATE KEY-----
EOF

# Thêm known_hosts cho github.com (để tránh github hiện thông báo lạ khi malware thử clone)
echo "github.com ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQCj7ndNxQowgcQnjshz..." > "${SSH_DIR}/known_hosts"

chmod 700 "${SSH_DIR}"
chmod 600 "${SSH_DIR}/id_rsa" "${SSH_DIR}/known_hosts"
echo "  - Planted: SSH Keys"

# =============================================================================
# 4. Git Config
# =============================================================================
cat <<EOF > "${USER_HOME}/.gitconfig"
[user]
	name = ${USER_NAME:-dev}
	email = ${USER_NAME:-dev}@company.internal.local
[credential]
	helper = store
EOF
echo "  - Planted: Git Config"

# =============================================================================
# 5. Bash History (Fake)
# Để nếu malware đọc .bash_history, nó sẽ thấy các lệnh như một dev bình thường
# =============================================================================
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
