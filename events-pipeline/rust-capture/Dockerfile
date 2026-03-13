FROM rust:1.88
RUN set -ex; \
  apt-get update; \
  apt-get install -y --no-install-recommends \
  pkg-config \
  libsasl2-dev
RUN DEBIAN_FRONTEND=noninteractive TZ=Etc/UTC apt-get -y install tzdata
ENV TZ=UTC
# Set the working directory
WORKDIR /app

ARG APP_RELEASE_VERSION

# Copy the application files into the image
COPY . .

# Build the application in release mode
RUN cargo build --release

# Set the command to run the binary
# CMD ["./target/release/events-pipeline"]
