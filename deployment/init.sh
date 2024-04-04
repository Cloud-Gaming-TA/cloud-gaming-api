#!/bin/bash
set -e

health_check() {
    # Endpoint for health check
    local ENDPOINT=$1

    while true; do
        # Send a request to the endpoint
        local RESPONSE=$(curl -k -s -o /dev/null -w "%{http_code}" $ENDPOINT)

        # Check if the endpoint responded with HTTP 200
        if [ $RESPONSE -eq 200 ]; then
            echo "Endpoint responded successfully. Exiting."
            return
        else
            echo "Endpoint check failed with HTTP status: $RESPONSE. Retrying in 5 seconds."
            sleep 5
        fi
    done
}



# get the directory where the script is located
SOURCE=${BASH_SOURCE[0]}
while [ -L "$SOURCE" ]; do # resolve $SOURCE until the file is no longer a symlink
  DIR=$( cd -P "$( dirname "$SOURCE" )" >/dev/null 2>&1 && pwd )
  SOURCE=$(readlink "$SOURCE")
  [[ $SOURCE != /* ]] && SOURCE=$DIR/$SOURCE # if $SOURCE was a relative symlink, we need to resolve it relative to the path where the symlink file was located
done

DIR=$( cd -P "$( dirname "$SOURCE" )" >/dev/null 2>&1 && pwd )

# set up secret file
PASSWORD_HASH_SECRET_KEY_FILE=$DIR/../api/secrets/password_hash_key.txt
JWT_SECRET_KEY_FILE=$DIR/../api/secrets/jwt_secret_key.txt
DB_ACCOUNT_SECRET_FILE=$DIR/db/secrets/db_account_password.txt
DB_AUTH_SECRET_FILE=$DIR/db/secrets/db_auth_password.txt
DB_GAMES_SECRET_FILE=$DIR/db/secrets/db_games_password.txt
SMTP_PASSWORD_FILE=$DIR/../middleware/steam-openid/secrets/smtp_password.txt
STEAM_API_KEY_FILE=$DIR/../middleware/mail/secrets/smtp_password.txt

mkdir -p $DIR/../api/secrets
mkdir -p $DIR/db/secrets
mkdir -p $DIR/../middleware/mail/secrets
mkdir -p $DIR../middleware/steam-openid/secrets

openssl rand -base64 128 > $PASSWORD_HASH_SECRET_KEY_FILE
openssl rand -base64 128 > $JWT_SECRET_KEY_FILE
openssl rand -base64 128 > $DB_ACCOUNT_SECRET_FILE
openssl rand -base64 128 > $DB_AUTH_SECRET_FILE
openssl rand -base64 128 > $DB_GAMES_SECRET_FILE

# access secret manager
gcloud secrets versions access latest --secret=STEAM_API_KEY > $STEAM_API_KEY_FILE
gcloud secrets versions access latest --secret=SMTP_CONFIG_PASSWORD > $SMTP_PASSWORD_FILE

echo $DIR
echo Starting certificate manager service. Please wait...

# run cert manager in the background
cd $DIR/../cert-manager && docker-compose down &&  docker-compose up --build -d

# cert manager health check
health_check "https://localhost:5500/health"

echo Certificate manager is up and running on localhost:5500

echo Starting the api. Please wait...
cd $DIR && docker-compose down && docker-compose up --build  -d
health_check "https://localhost:3000/health"
echo api is up and running on localhost:3000
echo success
exit 0




