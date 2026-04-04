# Just need an Ubuntu 24, doesn't mater the tag
FROM ubuntu:noble

# Đặt ENV để người dùng inject vào container khi họ tự build
ARG NODE_VERSION=20
ARG NVM_VERSION=0.39.7
ARG PACKAGE_MANAGER=npm
ARG USER_NAME=helvilette

# A full apt install of Ubuntu Desktop,
# as close as a real Desktop as possible
# Add some common "desktop" packages to fool simple anti-sandbox checks
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y \
    curl \
    wget \
    git \
    sudo \
    python3 \
    build-essential \
    bash \
    ca-certificates \
    apt-transport-https \
    nano \
    vim \
    gnupg \
    && apt-get clean && rm -rf /var/lib/apt/lists/*

# A user name... something that randomly generated, or ask user
# to import their real username (for fun)
RUN useradd -m -s /bin/bash ${USER_NAME} && \
    echo "${USER_NAME} ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers

USER ${USER_NAME}
WORKDIR /home/${USER_NAME}/app

# Run the scripts to generate honeypots credentials
# 1. AWS
RUN mkdir -p /home/${USER_NAME}/.aws && \
    echo "[default]\naws_access_key_id = AKIAIOSFODNN7EXAMPLE\naws_secret_access_key = wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY" > /home/${USER_NAME}/.aws/credentials

# 2. Kubeconfig
RUN mkdir -p /home/${USER_NAME}/.kube && \
    echo "apiVersion: v1\nclusters:\n- cluster:\n    server: https://10.0.0.1:6443\n  name: fake-k8s\ncontexts:\n- context:\n    cluster: fake-k8s\n    user: admin\n  name: default\ncurrent-context: default\nkind: Config\nusers:\n- name: admin\n  user:\n    token: fake-ey-jwt-token-super-secret" > /home/${USER_NAME}/.kube/config

# 3. SSH Keys
RUN mkdir -p /home/${USER_NAME}/.ssh && \
    echo "-----BEGIN OPENSSH PRIVATE KEY-----\nb3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW\nQyNTUxOQAAACBA1zK... (FAKE KEY) ...\n-----END OPENSSH PRIVATE KEY-----" > /home/${USER_NAME}/.ssh/id_rsa && \
    chmod 600 /home/${USER_NAME}/.ssh/id_rsa

# 4. Dummy VS Code config to make it look like a dev machine
RUN mkdir -p /home/${USER_NAME}/.config/Code/User && \
    echo "{ \"editor.formatOnSave\": true }" > /home/${USER_NAME}/.config/Code/User/settings.json

# Inject some juicy environment variables
ENV GEMINI_API_KEY="AIzaSy_FAKE_API_KEY_JUST_TRY_TO_STEAL_ME"
ENV STRIPE_SECRET_KEY="sk_live_FAKE_STRIPE_KEY"
ENV NPM_TOKEN="npm_FAKE_NPM_PUBLISH_TOKEN"

# Call nvm to install the destinated node version
RUN curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v${NVM_VERSION}/install.sh | bash && \
    export NVM_DIR="/home/${USER_NAME}/.nvm" && \
    [ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh" && \
    nvm install ${NODE_VERSION} && \
    nvm alias default ${NODE_VERSION} && \
    nvm use default

# Add nvm/node/npm to PATH
ENV NVM_DIR="/home/${USER_NAME}/.nvm"
ENV PATH="$NVM_DIR/versions/node/v${NODE_VERSION}/bin:$PATH"

# now, call the npm to install
CMD ["sh", "-c", "${PACKAGE_MANAGER} install --ignore-scripts=false"]
