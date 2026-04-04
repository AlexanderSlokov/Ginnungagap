# Just need an Ubuntu 24, doesn't mater the tag
FROM ubuntu:noble

ARG NODE_VERSION
ARG NVM_VERSION
ARG PACKAGE_MANAGER=npm
ARG USER_NAME=helvilette

# Đặt ENV để người dùng inject vào container khi họ tự build

# A full apt install of Ubuntu Desktop,
# as close as a real Desktop as possible

# A user name... something that randomly generated, or ask user
# to import their real username (for fun)

WORKDIR /home/$USER/app

# Run the scripts to generate honeypots credentials

# Call nvm to install the destinated node version

# now, call the npm to install