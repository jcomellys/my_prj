package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/jcomellys/voice-mac-agent/internal/brain"
)

// TaskDelegate runs an autonomous multi-step task and returns a final
// summary. Implemented by subagent.Runner; declared here as an interface so
// the tools package doesn't import subagent (which imports tools).
type TaskDelegate interface {
	Run(ctx context.Context, task string) (string, error)
}

// DelegateTask hands a hard, multi-step task to a stronger sub-agent that
// works autonomously (reads/writes files, runs allowlisted commands,
// browses, etc.) and reports back. The frontal agent uses this for things
// like "study this whole book", "build a small app", "analyze this circuit".
type DelegateTask struct {
	Delegate TaskDelegate
}

func NewDelegateTask(d TaskDelegate) *DelegateTask { return &DelegateTask{Delegate: d} }

func (DelegateTask) Spec() brain.ToolSpec {
	return brain.ToolSpec{
		Name:        "delegate_task",
		Description: "Entrega una tarea grande y de varios pasos a un agente experto que trabaja solo y devuelve un resultado. Úsalo para tareas que requieren leer/escribir archivos, programar, investigar a fondo, estudiar un documento completo, o múltiples acciones encadenadas. NO lo uses para comandos simples (abrir una app, decir la hora) — esos hazlos tú. Pasa una descripción clara y completa de lo que se debe lograr; el experto no ve la conversación, solo este texto.",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task": map[string]any{
					"type":        "string",
					"description": "Descripción completa y autónoma de la tarea a realizar.",
				},
			},
			"required":             []string{"task"},
			"additionalProperties": false,
		},
	}
}

func (t *DelegateTask) Execute(ctx context.Context, argsJSON string) (Result, error) {
	if t.Delegate == nil {
		return Result{}, fmt.Errorf("delegate_task: sub-agente no configurado")
	}
	var args struct {
		Task string `json:"task"`
	}
	if err := UnmarshalArgs(argsJSON, &args); err != nil {
		return Result{}, err
	}
	task := strings.TrimSpace(args.Task)
	if task == "" {
		return Result{}, fmt.Errorf("delegate_task: empty task")
	}
	out, err := t.Delegate.Run(ctx, task)
	if err != nil {
		return Result{}, err
	}
	return Result{Text: out}, nil
}
