# Just need an Ubuntu 24, doesn't mater the tag
FROM ubuntu:noble

# Set ENVs so users can inject into the container when they build it
ARG NODE_VERSION=20
ARG NVM_VERSION=0.39.7
ARG PACKAGE_MANAGER=npm
ARG USER_NAME=helvilette

# Thêm dòng này để truyền ARG vào ENV
ENV PACKAGE_MANAGER=${PACKAGE_MANAGER}

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
    openssh-client \
    && apt-get clean && rm -rf /var/lib/apt/lists/*

# A user name... something that randomly generated, or ask user
# to import their real username (for fun)
RUN useradd -m -s /bin/bash ${USER_NAME} && \
    echo "${USER_NAME} ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers

# Copy the dynamic credentials generator script into the container
COPY sandbox/generate_credentials.sh /usr/local/bin/generate_credentials.sh
RUN chmod +x /usr/local/bin/generate_credentials.sh

USER ${USER_NAME}
WORKDIR /home/${USER_NAME}/app

# Dummy VS Code config to make it look like a dev machine
RUN mkdir -p /home/${USER_NAME}/.config/Code/User && \
    echo "{ \"editor.formatOnSave\": true }" > /home/${USER_NAME}/.config/Code/User/settings.json

# Call nvm to install the destinated node version
RUN curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v${NVM_VERSION}/install.sh | bash && \
    export NVM_DIR="/home/${USER_NAME}/.nvm" && \
    [ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh" && \
    nvm install ${NODE_VERSION} && \
    nvm alias default ${NODE_VERSION} && \
    nvm use default

# The problem with 'sh' is it doesn't source ~/.bashrc or understand nvm setup
# We need to make sure bash is used to run the final command so nvm environment is loaded.
ENV NVM_DIR="/home/${USER_NAME}/.nvm"

# Make the generate_credentials.sh the Entrypoint so it randomizes keys
# EVERY SINGLE TIME the container is started (not just during build time)
ENTRYPOINT ["/usr/local/bin/generate_credentials.sh"]

# Call npm using bash so the NVM environment is properly loaded
CMD ["/bin/bash", "-c", "source $NVM_DIR/nvm.sh && ${PACKAGE_MANAGER} install"]
