#!/usr/bin/env bash
set -euo pipefail
# set -eux

# if [[ -z "${MOSQUITTO_USER_OPS:-}" || -z "${MOSQUITTO_PASSWORD_OPS:-}" || -z "${MOSQUITTO_USER_APP_WEB:-}" || -z "${MOSQUITTO_PASSWORD_APP_WEB:-}" ]]; then
#   echo "MOSQUITTO_USER_OPS/MOSQUITTO_PASSWORD_OPS/MOSQUITTO_USER_APP_WEB/MOSQUITTO_PASSWORD_APP_WEB must be set."
#   exit 1
# fi

cd ./config
touch password_file
chmod 0700 password_file
# chown root password_file

mosquitto_passwd -b password_file "${MOSQUITTO_USER_OPS}" "${MOSQUITTO_PASSWORD_OPS}"
mosquitto_passwd -b password_file "${MOSQUITTO_USER_APP_WEB}" "${MOSQUITTO_PASSWORD_APP_WEB}"

awk '{
  while (match($0, /\$\{[A-Za-z_][A-Za-z0-9_]*\}/)) {
    var = substr($0, RSTART + 2, RLENGTH - 3)
    gsub("\\$\\{" var "\\}", ENVIRON[var])
  }
  print
}' acl_file.tmpl > acl_file
chmod 0700 acl_file
