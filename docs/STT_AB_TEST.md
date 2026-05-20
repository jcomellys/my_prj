# A/B test: whisper-small vs whisper-medium

Test controlado, sin GPT, sin tokens. Compara los dos modelos sobre las
mismas grabaciones para decidir cuál usa el perfil `manos_libres` por
defecto. Si `medium` mejora claramente y mantiene latencia aceptable en
M4, se promueve. Si no, evaluamos STT cloud para tier premium.

## Prerequisitos

```bash
# Modelos
mkdir -p ~/.whisper-models
[ -f ~/.whisper-models/ggml-small.bin ]  || curl -L -o ~/.whisper-models/ggml-small.bin  https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small.bin
[ -f ~/.whisper-models/ggml-medium.bin ] || curl -L -o ~/.whisper-models/ggml-medium.bin https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-medium.bin

# Herramientas (sox y whisper-cli ya deberían estar; bc es nuevo)
brew install bc
```

`medium` pesa ~1.5 GB, tarda en bajar.

## Paso 1 — Grabar el corpus

```bash
cd ~/my_prj
bash scripts/record_phrases.sh
```

Te pide 10 frases una por una. Cada una se graba con el mismo `sox`
filter que usa el agente en vivo (16 kHz mono, corte por silencio de
~1.5 s). Genera `samples/01_*.wav` … `samples/10_*.wav` y un
`samples/manifest.txt` con `<wav> | <transcript esperado>`.

Habla normal, en español, cerca del mic. Si te equivocas, repite el
script — sobrescribe los archivos.

## Paso 2 — Correr el A/B

```bash
bash scripts/whisper_ab.sh samples/manifest.txt
```

Para cada archivo, muestra:
- Esperado.
- `small` con latencia y transcripción + ✅/❌ según coincidencia normalizada.
- `medium` con latencia y transcripción + ✅/❌.

Al final un resumen: `N/total coincidencias` y latencia media por modelo.

## Cómo interpretar

- **Coincidencias exactas** son una métrica estricta tras normalizar
  (lowercase, sin puntuación). No penaliza acentos ni capitalización.
  Sirve como contador rápido.
- **Lo cualitativo manda.** Lee cada par y juzga: ¿el modelo entendió la
  intención? ¿se inventa palabras? ¿confunde nombres propios?
- **Latencia importa para UX.** En el agente, latencia STT > 2 s rompe
  la sensación de conversación. Si `medium` te da 3-4 s por frase corta,
  no es viable para `manos_libres` aunque la calidad sea mejor.

## Criterio de decisión

Promovemos `medium` al perfil `manos_libres` si TODO lo siguiente se
cumple:

1. `medium` empata o supera a `small` en coincidencias en frases con
   nombres propios (las que `small` falla hoy: Newton, Wikipedia, etc.).
2. La latencia media de `medium` se queda **≤ 3.0 s** en M4 para
   utterances típicas (1-3 segundos de audio).
3. No introduce regresiones en frases simples ("qué hora es", "abre
   Chrome") — `medium` no debe ser peor que `small` en ningún caso
   común.

Si solo el (1) y (3) se cumplen pero (2) falla, dejamos `small` por
defecto y exponemos `medium` como opción en el perfil `premium`.

Si (1) falla, ni `medium` arregla el problema y hay que evaluar STT
cloud (OpenAI Whisper API, Deepgram, etc.).

## Reportar resultado

Cuando termines, pega al usuario:
- El resumen final del script (N/total y latencias).
- 2-3 ejemplos cualitativos representativos (un caso donde `medium`
  ganó claramente, uno donde fue igual, y uno donde `small` fue
  suficiente).
- Tu recomendación: ¿promover `medium`, mantener `small`, o evaluar STT
  cloud?

## Notas para el operador

- Las grabaciones quedan en `samples/`. Está gitignorado por defecto
  (añade `samples/` a `.gitignore` si no está).
- No subas las grabaciones al repo (privacidad).
- Si quieres comparar después con un modelo nuevo (large-v3, distil-large,
  etc.), agrega `MODEL_LARGE=...` y otro bloque al script. La interfaz
  está pensada para extender.
