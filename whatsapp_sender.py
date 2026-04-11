"""Modulo para enviar mensajes por WhatsApp usando Twilio."""

import os
from dotenv import load_dotenv
from twilio.rest import Client

load_dotenv()


def get_client():
    account_sid = os.getenv("TWILIO_ACCOUNT_SID")
    auth_token = os.getenv("TWILIO_AUTH_TOKEN")
    if not account_sid or not auth_token:
        raise ValueError(
            "Faltan credenciales de Twilio. "
            "Configura TWILIO_ACCOUNT_SID y TWILIO_AUTH_TOKEN en el archivo .env"
        )
    return Client(account_sid, auth_token)


def send_whatsapp_message(message):
    """Envia un mensaje por WhatsApp al numero configurado."""
    client = get_client()
    from_number = os.getenv("TWILIO_WHATSAPP_NUMBER")
    to_number = os.getenv("DESTINATION_PHONE")

    if not from_number or not to_number:
        raise ValueError(
            "Faltan numeros de telefono. "
            "Configura TWILIO_WHATSAPP_NUMBER y DESTINATION_PHONE en el archivo .env"
        )

    msg = client.messages.create(
        body=message,
        from_=from_number,
        to=to_number,
    )
    print(f"Mensaje enviado. SID: {msg.sid}")
    return msg


def send_greeting(name="amigo"):
    """Envia un saludo por WhatsApp."""
    message = f"Hola {name}! Este es un saludo automatico enviado desde el sistema."
    return send_whatsapp_message(message)


if __name__ == "__main__":
    send_greeting()
