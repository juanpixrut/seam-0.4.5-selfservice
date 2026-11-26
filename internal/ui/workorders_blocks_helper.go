package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	xolu "github.com/ha1tch/xolu/pkg/client"
)

const (
	workOrderBlocksEntity = "work_order_blocks"

	workOrderBlockPending    = "pending"
	workOrderBlockInProgress = "in_progress"
	workOrderBlockCompleted  = "completed"

	workOrderBlockDefinitionName = "seam_work_order_block_lifecycle_v1"
)

// WorkOrderBlock representa la ejecución de un bloque dentro de una WO.
//
// BlockIndex identifica el bloque dentro de la versión de la plantilla.
// PredecessorIndex vale -1 cuando no existe un predecesor.
//
// State será una copia del estado de su FSM, utilizada para consultas
// y presentación. Sus cambios deberán realizarse mediante la FSM.
type WorkOrderBlock struct {
	ID               int64
	WorkOrderID      int64
	BlockIndex       int
	PredecessorIndex int
	MachineID        int64
	State            string
	Values           map[string]any
	StartedAt        string
	CompletedAt      string
}

// workOrderBlockPredecessors valida los bloques y convierte las
// referencias por nombre en índices de la plantilla.
//
// Permite:
//   - Varios bloques sin predecesor.
//   - Varios bloques con el mismo predecesor.
//   - Referencias a bloques ubicados después en el arreglo.
//
// Rechaza:
//   - Nombres vacíos o repetidos.
//   - Predecesores inexistentes.
//   - Un bloque como su propio predecesor.
//   - Dependencias circulares.
func workOrderBlockPredecessors(blocks []TemplateBlock) ([]int, error) {
	normalize := func(value string) string {
		return strings.ToLower(strings.TrimSpace(value))
	}

	indices := make(map[string]int, len(blocks))

	for i, block := range blocks {
		name := normalize(block.Name)

		if name == "" {
			return nil, fmt.Errorf("block %d has an empty name", i)
		}

		if previous, exists := indices[name]; exists {
			return nil, fmt.Errorf("blocks %d and %d have the same name", previous, i)
		}

		indices[name] = i
	}

	predecessors := make([]int, len(blocks))

	for i, block := range blocks {
		predecessors[i] = -1

		name := normalize(block.Predecessor)
		if name == "" {
			continue
		}

		predecessor, exists := indices[name]
		if !exists {
			return nil, fmt.Errorf("block %q references unknown predecessor %q", block.Name, block.Predecessor)
		}

		if predecessor == i {
			return nil, fmt.Errorf("block %q references itself", block.Name)
		}

		predecessors[i] = predecessor
	}

	// 0: no visitado.
	// 1: recorriéndose actualmente.
	// 2: validado.
	visited := make([]uint8, len(blocks))

	var visit func(int) error

	visit = func(index int) error {
		switch visited[index] {
		case 1:
			return fmt.Errorf("dependency cycle involving block %q", blocks[index].Name)

		case 2:
			return nil
		}

		visited[index] = 1

		if predecessor := predecessors[index]; predecessor >= 0 {
			if err := visit(predecessor); err != nil {
				return err
			}
		}

		visited[index] = 2
		return nil
	}

	for i := range blocks {
		if err := visit(i); err != nil {
			return nil, err
		}
	}

	return predecessors, nil
}

// workOrderBlockMachineSpec define el ciclo de vida común a todos
// los bloques. Cada bloque de cada WO tendrá su propia instancia.
func workOrderBlockMachineSpec() xolu.MachineSpec {
	return xolu.MachineSpec{
		Name: workOrderBlockDefinitionName,

		Description: "Work order block lifecycle",

		Initial: workOrderBlockPending,

		Determinism: "firstmatch",

		States: map[string]xolu.StateDef{
			workOrderBlockPending: {
				Terminal: false,
			},
			workOrderBlockInProgress: {
				Terminal: false,
			},
			workOrderBlockCompleted: {
				Terminal: true,
			},
		},

		Transitions: []xolu.TransitionDef{
			{
				From:  json.RawMessage(`"pending"`),
				Input: "start",
				To:    workOrderBlockInProgress,
			},
			{
				From:  json.RawMessage(`"in_progress"`),
				Input: "complete",
				To:    workOrderBlockCompleted,
			},
		},
	}
}

// ensureWorkOrderBlockDefinition busca la definición compartida.
// Si todavía no existe, la crea.
//
// Sigue el patrón utilizado por ensureIssueFSMDefinition.
func (h *UIHandler) ensureWorkOrderBlockDefinition(ctx context.Context) (int64, error) {
	definitions, err := h.xoluClient.ListMachineDefs(ctx)
	if err != nil {
		return 0, err
	}

	var definitionID int64

	for _, definition := range definitions {
		if definition.Name != workOrderBlockDefinitionName {
			continue
		}

		if definitionID != 0 {
			return 0, fmt.Errorf("multiple block FSM definitions named %q", workOrderBlockDefinitionName)
		}

		definitionID = definition.ID
	}

	if definitionID != 0 {
		return definitionID, nil
	}

	result, err := h.xoluClient.CreateMachineDef(ctx, workOrderBlockMachineSpec())
	if err != nil {
		return 0, err
	}

	return result.ID, nil
}

// workOrderCreationUncertainError identifies a commit whose outcome could
// not be confirmed. Its machines must be preserved because the order may
// already have been saved.
type workOrderCreationUncertainError struct {
	OrderID int64
	Cause   error
}

func (e *workOrderCreationUncertainError) Error() string {
	return fmt.Sprintf("could not confirm creation of work order %d: %v", e.OrderID, e.Cause)
}

func (e *workOrderCreationUncertainError) Unwrap() error {
	return e.Cause
}

// prepareWorkOrderBlocks creates one FSM instance per template block and
// returns the entity inserts for the order's commit.
//
// It does not save block entities. On failure, machineIDs contains every
// successfully created machine whose ID is known, so the caller can clean
// them up before returning.
func (h *UIHandler) prepareWorkOrderBlocks(ctx context.Context, orderID int64, template WorkOrderTemplate) (inserts []xolu.CommitAppend, machineIDs []int64, err error) {
	predecessors, err := workOrderBlockPredecessors(template.Blocks)
	if err != nil {
		return nil, nil, err
	}

	if len(template.Blocks) == 0 {
		return nil, nil, nil
	}

	definitionID, err := h.ensureWorkOrderBlockDefinition(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("ensure block FSM definition: %w", err)
	}

	inserts = make([]xolu.CommitAppend, 0, len(template.Blocks))
	machineIDs = make([]int64, 0, len(template.Blocks))

	now := time.Now().UTC().Format(time.RFC3339)

	for index, block := range template.Blocks {
		ref := fmt.Sprintf("work_order:%d:block:%d", orderID, index)

		machine, createErr := h.xoluClient.CreateMachine(ctx,
			xolu.CreateMachineRequest{
				Definition: definitionID,
				Ref:        ref,
			},
		)

		if createErr != nil {
			// A lost response may leave a machine whose ID is unknown.
			// Its ref allows operators to locate it without guessing.
			h.logger.Error("failed to create block FSM", "work_order_id", orderID, "block_index", index, "machine_ref", ref, "error", createErr)

			return nil, machineIDs, fmt.Errorf("create FSM for block %d: %w", index, createErr)
		}

		if machine == nil || machine.ID <= 0 {
			return nil, machineIDs, fmt.Errorf("invalid FSM creation response for block %d", index)
		}

		machineIDs = append(machineIDs, machine.ID)

		if machine.State != workOrderBlockPending {
			return nil, machineIDs, fmt.Errorf("block %d FSM started in unexpected state %q", index, machine.State)
		}

		blockData := map[string]any{
			"work_order_id":     orderID,
			"template_id":       template.ID,
			"template_version":  template.Version,
			"block_index":       index,
			"predecessor_index": predecessors[index],
			"name":              strings.TrimSpace(block.Name),
			"machine_id":        machine.ID,
			"state":             machine.State,
			"values":            map[string]any{},
			"started_at":        "",
			"completed_at":      "",
			"created_at":        now,
			"updated_at":        now,
		}

		inserts = append(inserts, xolu.CommitAppend{
			Entity: workOrderBlocksEntity,
			Data:   blockData,
		})
	}

	return inserts, machineIDs, nil
}

// discardWorkOrderBlockMachines performs best-effort cleanup only when
// their block entities have not been committed.
//
// The detached context keeps cleanup possible after the HTTP request is
// cancelled. Failures are logged so abandoned machines can be inspected.
func (h *UIHandler) discardWorkOrderBlockMachines(ctx context.Context, orderID int64, machineIDs []int64) {
	if len(machineIDs) == 0 {
		return
	}

	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancel()

	for _, machineID := range machineIDs {
		if err := h.xoluClient.DeleteMachine(cleanupCtx, machineID); err != nil {
			h.logger.Error("failed to clean up uncommitted block FSM", "work_order_id", orderID, "machine_id", machineID, "error", err)
		}
	}
}
