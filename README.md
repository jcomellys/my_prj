# my_prj - WhatsApp Greeting & Generators Monitor

Sistema para enviar saludos y reportes automáticos del estado de generadores de fortuna por WhatsApp usando Twilio.

## Configuración

1. Crea una cuenta gratuita en [Twilio](https://www.twilio.com/try-twilio)
2. En la consola de Twilio, activa el sandbox de WhatsApp:
   - Ve a **Messaging > Try it out > Send a WhatsApp message**
   - Sigue las instrucciones para conectar tu número (enviar un código al número de Twilio)
3. Copia el archivo de configuración y complétalo con tus credenciales:
   ```bash
   cp .env.example .env
   ```
4. Edita `.env` con tus datos de Twilio (Account SID, Auth Token y números)
5. Instala las dependencias:
   ```bash
   pip install -r requirements.txt
   ```

## Uso

### Enviar un saludo
```bash
python whatsapp_sender.py
```

### Iniciar monitoreo de generadores
```bash
python generators_monitor.py
```
Esto enviará un reporte cada 60 minutos por WhatsApp.
