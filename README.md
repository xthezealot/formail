# Formail

Formail allows your static websites to send forms by email, from the client-side, without exposing your SMTP credentials.  
It is a self-hosted, open-source alternative to Formspree.

## Install

### Docker

```sh
docker run -d \
	--name formail \
	-p 8080:8080 \
	-e SECRET=<SECRET_KEY> \
	--restart unless-stopped \
	thezealot/formail
```

## Usage

1. ### Encode your configuration

   Before sending form data, you need to encrypt your form configuration using the `/encrypt` endpoint:

   ```sh
   curl https://<HOST>/encrypt?key=<SECRET_KEY> \
   		-H "Content-Type: application/json" \
   		-d '{
   			"smtpHost":"<SMTP_HOST>",
   			"smtpPort":<SMTP_PORT>,
   			"smtpUsername":"<SMTP_USER>",
   			"smtpPassword":"<SMTP_PASSWORD>",
   			"from":"<FROM_EMAIL>","to":["<TO_EMAIL>"],
   			"subject":"<SUBJECT>",
   			"fields":["<FIELD1>","<FIELD2>"]
   			}'
   ```

   The response body will contain the encrypted JSON configuration string that you will include in your static website code for future client-side requests.

2. ### Send your form data

   Once you have the encrypted configuration, send your form data to the `/` endpoint:

   ```json
   POST /
   Content-Type: application/json

   {
   	"config": "<ENCRYPTED_CONFIG>",
   	"fields": {
   		"name": "John Doe",
   		"email": "john.doe@example.com",
   		"message": "This is a test message."
   	}
   }
   ```

   Formail will decrypt the configuration, match the form data to the fields, and send an email with the collected data.
