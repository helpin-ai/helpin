FROM python:3.11-bookworm
ENV TZ="UTC"
ENV PYTHONUNBUFFERED=1

WORKDIR /app/
# Add these lines to your Dockerfile
RUN apt-get update && apt-get install -y \
    postgresql-client \
    libpq-dev \
    ca-certificates

# Install Poetry
RUN curl -sSL https://install.python-poetry.org | POETRY_HOME=/opt/poetry python && \
    cd /usr/local/bin && \
    ln -s /opt/poetry/bin/poetry && \
    poetry config virtualenvs.create false

# Copy poetry.lock* in case it doesn't exist in the repo
COPY ./pyproject.toml ./poetry.lock* /app/
# Allow installing dev dependencies to run tests
ARG INSTALL_DEV=true
RUN bash -c "if [ $INSTALL_DEV == 'true' ] ; then poetry install --no-root ; else poetry install --no-root --no-dev ; fi"

ARG APP_RELEASE_VERSION

COPY . /app
ENV PYTHONPATH=/app
ENTRYPOINT ["python3", "main.py"]
