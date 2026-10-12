#!/bin/sh
# Renders alertmanager.yml from the environment, then starts Alertmanager
# (or, with "check", validates the rendered config with amtool and exits).
#
# Env: ALERT_EMAIL_TO (recipient; required when APP_ENV=prod, otherwise it
# defaults to a local address that Mailpit catches), SMTP_HOST, SMTP_PORT,
# SMTP_FROM, SMTP_USER, SMTP_PASSWORD (same as the API's mailer),
# ALERT_SMTP_REQUIRE_TLS (true|false; defaults to true in prod, false
# elsewhere because Mailpit speaks plain SMTP).
set -eu

fail() { echo "alertmanager config: $*" >&2; exit 1; }

if [ -z "${ALERT_EMAIL_TO:-}" ]; then
  [ "${APP_ENV:-dev}" = "prod" ] && fail "ALERT_EMAIL_TO is required in prod"
  ALERT_EMAIL_TO=alerts@fluxboard.local
fi
[ -n "${SMTP_HOST:-}" ] || fail "SMTP_HOST is required"
[ -n "${SMTP_FROM:-}" ] || fail "SMTP_FROM is required"
SMTP_PORT=${SMTP_PORT:-1025}
SMTP_USER=${SMTP_USER:-}
if [ "${APP_ENV:-dev}" = "prod" ]; then
  ALERT_SMTP_REQUIRE_TLS=${ALERT_SMTP_REQUIRE_TLS:-true}
else
  ALERT_SMTP_REQUIRE_TLS=${ALERT_SMTP_REQUIRE_TLS:-false}
fi

case "$ALERT_SMTP_REQUIRE_TLS" in
  true | false) ;;
  *) fail "ALERT_SMTP_REQUIRE_TLS must be true or false" ;;
esac
case "$SMTP_PORT" in
  '' | *[!0-9]*) fail "SMTP_PORT must be a number" ;;
esac
# Values are substituted into double-quoted YAML with sed; refuse characters
# that would break either instead of escaping them.
for v in "$ALERT_EMAIL_TO" "$SMTP_HOST" "$SMTP_FROM" "$SMTP_USER"; do
  case "$v" in
    *'"'* | *'\'* | *'|'* | *'&'*) fail "SMTP/alert values must not contain \" \ | or &" ;;
  esac
done

umask 077
printf '%s' "${SMTP_PASSWORD:-}" > /tmp/smtp_password
sed -e "s|__SMTP_HOST__|$SMTP_HOST|g" \
    -e "s|__SMTP_PORT__|$SMTP_PORT|g" \
    -e "s|__SMTP_FROM__|$SMTP_FROM|g" \
    -e "s|__SMTP_USER__|$SMTP_USER|g" \
    -e "s|__ALERT_SMTP_REQUIRE_TLS__|$ALERT_SMTP_REQUIRE_TLS|g" \
    -e "s|__ALERT_EMAIL_TO__|$ALERT_EMAIL_TO|g" \
    /etc/alertmanager/alertmanager.yml > /tmp/alertmanager.yml

if [ "${1:-}" = "check" ]; then
  exec amtool check-config /tmp/alertmanager.yml
fi
exec /bin/alertmanager --config.file=/tmp/alertmanager.yml --storage.path=/alertmanager "$@"
