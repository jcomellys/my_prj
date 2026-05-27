# Fase 1 incremento 2: narracion a media tarea

## Problema

En el incremento 1, `delegate_task` bloquea al agente frontal hasta que el
sub-agente termina. Eso es correcto para aislar el trabajo autonomo, pero la
experiencia queda silenciosa durante tareas largas: el usuario no sabe si el
agente esta leyendo, escribiendo, bloqueado o gastando tokens. La solucion debe
mantener el barge-in actual, no convertir al sub-agente en un segundo narrador
costoso, y no debilitar las reglas de seguridad de herramientas.

## Opciones

| Opcion | Como funciona | Costo | Complejidad | Riesgo de barge-in |
| --- | --- | --- | --- | --- |
| Canal Go de eventos de progreso | `subagent.Runner` emite eventos estructurados por canal: inicio, ronda, tool ok/error, fin. El frontal los resume con frases fijas y baratas. | Bajo: no usa tokens extra. | Media: requiere cablear canal y TTS concurrente. | Bajo si el canal escucha `ctx.Done()` y el TTS usa el mismo cancel. |
| Resumen periodico cada N rondas | Cada N rondas, el sub-agente pide al brain un resumen corto del estado y el frontal lo lee. | Medio/alto: agrega llamadas o tokens. | Baja/media: se apoya en el brain existente. | Medio: mas latencia y mas texto que cortar. |
| Tool `report_progress` | El sub-agente decide llamar una herramienta para reportar progreso al frontal. | Variable: depende del modelo; puede gastar tokens y abusarse. | Media: requiere nueva tool y prompt. | Medio/alto: el modelo puede reportar de mas, de menos, o durante momentos inoportunos. |

## Propuesta

Usar un canal Go de eventos de progreso emitidos por el runner, con narracion
deterministica en el frontal. Es la opcion mas barata y mas controlable:
aprovecha eventos que ya existen en logs (`subagent.start`, `subagent.round`,
`subagent.tool.ok`, `subagent.tool.error`, `subagent.done`) y no pide al modelo
que genere texto adicional.

La regla de producto seria:

- Al delegar: decir una frase fija, por ejemplo "Lo paso al sub-agente; te voy
  avisando."
- Durante la tarea: narrar como maximo un evento cada 8-12 segundos, coalescido,
  por ejemplo "Sigo trabajando; ya escribi un archivo" o "Estoy revisando el
  siguiente paso."
- Al terminar: leer el resumen final normal del frontal.
- Si hay barge-in: cancelar `ctx`, detener el TTS en curso y dejar de consumir
  eventos.

## Bosquejo de API

```go
type ProgressKind string

const (
	ProgressStarted ProgressKind = "started"
	ProgressRound   ProgressKind = "round"
	ProgressToolOK  ProgressKind = "tool_ok"
	ProgressToolErr ProgressKind = "tool_error"
	ProgressDone    ProgressKind = "done"
)

type ProgressEvent struct {
	Kind  ProgressKind
	Round int
	Tool  string
	Text  string
}

type Runner struct {
	Brain     brain.Brain
	Tools     *tools.Registry
	System    string
	MaxRounds int
	Progress  chan<- ProgressEvent // optional, non-blocking send
}
```

El runner enviaria eventos con una funcion auxiliar no bloqueante:

```go
func (r *Runner) emit(ctx context.Context, ev ProgressEvent) {
	if r.Progress == nil {
		return
	}
	select {
	case r.Progress <- ev:
	case <-ctx.Done():
	default:
	}
}
```

El frontal tendria una goroutine por delegacion que escucha eventos, aplica
rate limit y llama al TTS con frases predefinidas. Esa goroutine muere cuando
se cierra el canal, llega `ctx.Done()` o termina `delegate_task`.

## Criterios de aceptacion

- Una tarea delegada larga produce al menos una narracion intermedia antes del
  resumen final.
- Una tarea delegada corta no habla de mas: inicio y resumen final bastan.
- Ctrl+Alt+Space cancela la tarea y la narracion en menos de un segundo.
- No hay llamadas extra al brain solo para progreso.
- Los tests cubren coalescing/rate limit y cancelacion sin goroutines colgadas.
