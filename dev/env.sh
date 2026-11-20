# Sourced by the dev scripts: points Libredesk at the project-local services.
# Libredesk reads LIBREDESK_<SECTION>__<KEY> env vars on top of its config file.
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "$ROOT/local/secrets.env"

export LIBREDESK_APP__ENV=dev
export LIBREDESK_APP__ENCRYPTION_KEY="$ENCRYPTION_KEY"
export LIBREDESK_APP__CHECK_UPDATES=false
export LIBREDESK_APP__SERVER__ADDRESS="127.0.0.1:9000"
# Plain http on localhost, so cookies can't be "secure"-only here
export LIBREDESK_APP__SERVER__DISABLE_SECURE_COOKIES=true
export LIBREDESK_DB__HOST=127.0.0.1
export LIBREDESK_DB__PORT=5434
export LIBREDESK_DB__USER=libredesk
export LIBREDESK_DB__PASSWORD="$DB_PASSWORD"
export LIBREDESK_DB__DATABASE=libredesk
export LIBREDESK_REDIS__ADDRESS="127.0.0.1:6380"
export LIBREDESK_UPLOAD__FS__UPLOAD_PATH="$ROOT/local/uploads"
export LIBREDESK_SYSTEM_USER_PASSWORD="$SYSTEM_USER_PASSWORD"

LIBREDESK_DIR="$ROOT/libredesk"
LIBREDESK_CONFIG="$ROOT/local/config.toml"
