package api

import (
	"fmt"
	"net/http"

	"github.com/ha1tch/queryfy"
	"github.com/ha1tch/seam-ui/internal/fsm"
	"github.com/ha1tch/seam-ui/internal/validation"
	xolu "github.com/ha1tch/xolu/pkg/client"
)

// FSMHandler handles FSM-related API endpoints.
type FSMHandler struct {
	*Handler
	Evaluator *fsm.Evaluator
}

// NewFSMHandler creates a new FSM handler.
func NewFSMHandler(h *Handler, evaluator *fsm.Evaluator) *FSMHandler {
	return &FSMHandler{
		Handler:   h,
		Evaluator: evaluator,
	}
}

// List handles GET /api/v1/fsm
func (h *FSMHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := h.XOLUWithTenant(r)

	params := GetPaginationParams(r)

	machinesResult, err := client.List(ctx, "fsm_machines", &xolu.ListParams{
		Limit:  params.Limit,
		Offset: params.Offset,
		Sort:   params.Sort,
	})
	if err != nil {
		WriteXOLUError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, NewListResponse(machinesResult.Entities, machinesResult.TotalItems, params.Limit, params.Offset))
}

// Get handles GET /api/v1/fsm/{id}
func (h *FSMHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := h.XOLUWithTenant(r)

	id, err := GetIDFromPath(r, 3)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_id", "Invalid FSM ID")
		return
	}

	machine, err := client.Get(ctx, "fsm_machines", id)
	if err != nil {
		WriteXOLUError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, machine)
}

// Create handles POST /api/v1/fsm
func (h *FSMHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := h.XOLUWithTenant(r)

	var data map[string]any
	if err := ReadJSON(r, &data); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	if err := queryfy.Validate(data, validation.FSMCreateSchema); err != nil {
		WriteValidationError(w, err)
		return
	}

	machine, err := client.Create(ctx, "fsm_machines", data)
	if err != nil {
		WriteXOLUError(w, err)
		return
	}

	// Load into evaluator
	h.loadMachineIntoEvaluator(machine)

	WriteJSON(w, http.StatusCreated, machine)
}

// Update handles PUT /api/v1/fsm/{id}
func (h *FSMHandler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := h.XOLUWithTenant(r)

	id, err := GetIDFromPath(r, 3)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_id", "Invalid FSM ID")
		return
	}

	var data map[string]any
	if err := ReadJSON(r, &data); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	if err := queryfy.Validate(data, validation.FSMUpdateSchema); err != nil {
		WriteValidationError(w, err)
		return
	}

	machine, err := client.Update(ctx, "fsm_machines", id, data)
	if err != nil {
		WriteXOLUError(w, err)
		return
	}

	// Reload in evaluator
	h.Evaluator.UnloadMachine(id)
	h.loadMachineIntoEvaluator(machine)

	WriteJSON(w, http.StatusOK, machine)
}

// Delete handles DELETE /api/v1/fsm/{id}
func (h *FSMHandler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := h.XOLUWithTenant(r)

	id, err := GetIDFromPath(r, 3)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_id", "Invalid FSM ID")
		return
	}

	// Check if any asset types reference this FSM
	query := fmt.Sprintf("SELECT COUNT(*) as count FROM asset_types WHERE fsm_id = %d", id)
	result, err := client.OQL(ctx, query)
	if err != nil {
		WriteXOLUError(w, err)
		return
	}

	if len(result.Data) > 0 {
		if count, ok := result.Data[0]["count"].(float64); ok && count > 0 {
			WriteError(w, http.StatusConflict, "fsm_in_use",
				fmt.Sprintf("Cannot delete FSM: %d asset types are using it", int(count)))
			return
		}
	}

	if err := client.Delete(ctx, "fsm_machines", id); err != nil {
		WriteXOLUError(w, err)
		return
	}

	h.Evaluator.UnloadMachine(id)

	w.WriteHeader(http.StatusNoContent)
}

// GetStates handles GET /api/v1/fsm/{id}/states
// Returns all states for an FSM.
func (h *FSMHandler) GetStates(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := h.XOLUWithTenant(r)

	id, err := GetIDFromPath(r, 3)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_id", "Invalid FSM ID")
		return
	}

	machine, err := client.Get(ctx, "fsm_machines", id)
	if err != nil {
		WriteXOLUError(w, err)
		return
	}

	states, ok := machine.Data["states"]
	if !ok {
		WriteJSON(w, http.StatusOK, map[string]any{"states": []any{}})
		return
	}

	WriteJSON(w, http.StatusOK, map[string]any{"states": states})
}

// Visualize handles GET /api/v1/fsm/{id}/visualize
// Returns FSM in a format suitable for visualization (e.g., Mermaid diagram).
func (h *FSMHandler) Visualize(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := h.XOLUWithTenant(r)

	id, err := GetIDFromPath(r, 3)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_id", "Invalid FSM ID")
		return
	}

	format := GetQueryString(r, "format", "mermaid")

	machine, err := client.Get(ctx, "fsm_machines", id)
	if err != nil {
		WriteXOLUError(w, err)
		return
	}

	switch format {
	case "mermaid":
		diagram := h.generateMermaidDiagram(machine)
		WriteJSON(w, http.StatusOK, map[string]any{
			"format":  "mermaid",
			"diagram": diagram,
		})
	case "json":
		WriteJSON(w, http.StatusOK, machine)
	default:
		WriteError(w, http.StatusBadRequest, "invalid_format", "Supported formats: mermaid, json")
	}
}

// Simulate handles POST /api/v1/fsm/{id}/simulate
// Simulates FSM transitions without persisting state changes.
func (h *FSMHandler) Simulate(w http.ResponseWriter, r *http.Request) {
	id, err := GetIDFromPath(r, 3)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_id", "Invalid FSM ID")
		return
	}

	var simData struct {
		CurrentState string         `json:"current_state"`
		Event        map[string]any `json:"event"`
	}
	if err := ReadJSON(r, &simData); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	machine, ok := h.Evaluator.GetMachine(id)
	if !ok {
		WriteError(w, http.StatusNotFound, "fsm_not_loaded", "FSM not loaded in evaluator")
		return
	}

	currentState := simData.CurrentState
	if currentState == "" {
		currentState = machine.Initial
	}

	// Find matching transitions from current state
	state, ok := machine.States[currentState]
	if !ok {
		WriteError(w, http.StatusBadRequest, "invalid_state", "Current state not found in FSM")
		return
	}

	possibleTransitions := []map[string]any{}
	for _, trans := range state.Transitions {
		possibleTransitions = append(possibleTransitions, map[string]any{
			"to":        trans.To,
			"condition": trans.Condition,
			"priority":  trans.Priority,
		})
	}

	// Build evaluation context
	evalCtx := &fsm.EvaluationContext{
		CurrentState: currentState,
		Event: &fsm.Event{
			TriggerType: getString(simData.Event, "trigger_type"),
			Value:       simData.Event,
		},
	}

	result, err := h.Evaluator.EvaluateMachine(machine, evalCtx)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "evaluation_error", err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"current_state":        currentState,
		"would_transition":     result.Triggered,
		"new_state":            result.ToState,
		"possible_transitions": possibleTransitions,
	})
}

// loadMachineIntoEvaluator converts an xolu entity to fsm.Machine and loads it.
func (h *FSMHandler) loadMachineIntoEvaluator(entity *xolu.Entity) {
	if h.Evaluator == nil {
		return
	}

	machine := &fsm.Machine{
		ID:      entity.ID,
		Name:    getString(entity.Data, "name"),
		Initial: getString(entity.Data, "initial"),
		States:  make(map[string]*fsm.State),
	}

	// Parse states
	if states, ok := entity.Data["states"].(map[string]any); ok {
		for name, stateData := range states {
			if sd, ok := stateData.(map[string]any); ok {
				state := &fsm.State{
					Name:    name,
					Label:   getString(sd, "label"),
					Color:   getString(sd, "color"),
					IsFinal: getBool(sd, "is_final"),
				}

				// Parse transitions
				if transitions, ok := sd["transitions"].([]any); ok {
					for _, t := range transitions {
						if td, ok := t.(map[string]any); ok {
							trans := &fsm.Transition{
								To: getString(td, "to"),
							}
							if priority, ok := td["priority"].(float64); ok {
								trans.Priority = int(priority)
							}
							if cond, ok := td["condition"].(map[string]any); ok {
								trans.Condition = parseFSMCondition(cond)
							}
							state.Transitions = append(state.Transitions, trans)
						}
					}
				}

				machine.States[name] = state
			}
		}
	}

	h.Evaluator.LoadMachine(machine)
}

// generateMermaidDiagram creates a Mermaid state diagram from FSM data.
func (h *FSMHandler) generateMermaidDiagram(entity *xolu.Entity) string {
	diagram := "stateDiagram-v2\n"

	initial := getString(entity.Data, "initial")
	if initial != "" {
		diagram += fmt.Sprintf("    [*] --> %s\n", initial)
	}

	states, ok := entity.Data["states"].(map[string]any)
	if !ok {
		return diagram
	}

	for name, stateData := range states {
		sd, ok := stateData.(map[string]any)
		if !ok {
			continue
		}

		// Add state label if different from name
		if label := getString(sd, "label"); label != "" && label != name {
			diagram += fmt.Sprintf("    %s : %s\n", name, label)
		}

		// Add transitions
		if transitions, ok := sd["transitions"].([]any); ok {
			for _, t := range transitions {
				if td, ok := t.(map[string]any); ok {
					to := getString(td, "to")
					diagram += fmt.Sprintf("    %s --> %s\n", name, to)
				}
			}
		}

		// Mark final states
		if getBool(sd, "is_final") {
			diagram += fmt.Sprintf("    %s --> [*]\n", name)
		}
	}

	return diagram
}

func parseFSMCondition(m map[string]any) *fsm.Condition {
	cond := &fsm.Condition{
		Type: fsm.ConditionType(getString(m, "type")),
	}

	cond.Field = getString(m, "field")
	cond.Target = getString(m, "target")
	cond.Trigger = getString(m, "trigger")

	if threshold, ok := m["threshold"].(float64); ok {
		cond.Threshold = &threshold
	}

	if values, ok := m["values"].([]any); ok {
		for _, v := range values {
			if s, ok := v.(string); ok {
				cond.Values = append(cond.Values, s)
			}
		}
	}

	// Parse sub-conditions for and/or types
	if subConds, ok := m["conditions"].([]any); ok {
		for _, sc := range subConds {
			if scm, ok := sc.(map[string]any); ok {
				cond.Conditions = append(cond.Conditions, parseFSMCondition(scm))
			}
		}
	}

	return cond
}
