# Configurar Chrome para el agente — una sola vez

Para que el agente pueda **leer páginas web y hacer click en enlaces** vía
voz, Chrome necesita un toggle activado por seguridad. Es un click, una
sola vez.

## Pasos (en tu Mac)

1. Abre Google Chrome.
2. En la barra de menú superior, click en **View** (o **Visualización** si
   tu Mac está en español).
3. Submenú **Developer** (o **Desarrollador**).
4. Marca **Allow JavaScript from Apple Events** (o **Permitir JavaScript
   desde eventos de Apple**).
5. Listo. La opción persiste entre reinicios de Chrome.

## Verificar que funcionó

En Terminal:

```bash
osascript -e 'tell application "Google Chrome" to execute active tab of front window javascript "1+1"'
```

Debe devolver `2`. Si en cambio devuelve `execution error: ...JavaScript
through Apple Events is turned off...`, el toggle no se aplicó. Repite los
pasos.

## Por qué el toggle existe

Chrome lo desactiva por defecto desde 2019 como protección contra apps
maliciosas que podrían robar sesiones leyendo cookies o pestañas. Al
activarlo le das permiso EXPLÍCITO a aplicaciones que usen AppleScript
(como nuestro agente, vía osascript) a ejecutar JavaScript en tus
pestañas.

Trade-off honesto: el toggle reduce un poco tu seguridad contra apps
maliciosas instaladas con permiso de AppleScript. En la práctica, si una
app llegó a poder ejecutarte AppleScript ya tienes problemas mayores.

## Qué desbloquea esto

Una vez activado, tu agente puede:

- "Navega a Wikipedia y léeme el primer párrafo sobre Newton."
- "Busca en Google 'circuitos RLC' y léeme el primer resultado."
- "Llena el formulario con mi nombre Juan."
- "Haz click en el enlace que dice Comprar."
- "Cierra esta pestaña."

Todo por voz, sin teclado ni mouse.

## Alternativa (fase futura)

En una fase posterior agregaremos control vía Chrome DevTools Protocol
que **no requiere este toggle**. Por ahora, AppleScript+JS es el camino
más simple y suficiente para 80% de los casos.
