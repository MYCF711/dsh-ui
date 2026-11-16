package dsrightbar

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// ledgerKind mirrors TrajectoryCellKind (trajectory-record.ts:8-16).
type ledgerKind string

const (
	ledgerSystem    ledgerKind = "system"
	ledgerUser      ledgerKind = "user"
	ledgerContext   ledgerKind = "context"
	ledgerCompacted ledgerKind = "compacted"
	ledgerMessage   ledgerKind = "message"
	ledgerTool      ledgerKind = "tool"
	ledgerSubtool   ledgerKind = "subtool"
)

// ledgerUsage retains presence independently for all five provider buckets.
// See TokenUsage (dsh-llm/src/types.ts:182-196).
type ledgerUsage struct {
	Input, CacheRead, CacheWrite, Output, Reasoning *int64
}

// ledgerRequest mirrors RequestView (request-inspection.ts:126-176).
type ledgerRequest struct {
	ID                                                 string
	Number, Turn, Step                                 int
	Status, Purpose, Provider, Model, Error, ErrorCode string
	Retry, MaxRetries, RetryDelayMS                    *int64
	StartedAt, CompletedAt                             *int64
	Options                                            string
	Usage                                              *ledgerUsage
	CumulativeUsage                                    *ledgerUsage
	ResultSeq                                          int
	Compaction                                         bool
}

// ledgerRecord is the Go Inspector projection of TrajectoryCellProps
// (trajectory-record.ts:39-107). Nil timestamps mean that no source frame time exists.
type ledgerRecord struct {
	ID                                   string
	Index, Turn, Step                    int
	Group                                string
	Kind                                 ledgerKind
	Text, Preview, Input, Output         string
	Thinking, Source, Schema, Options    string
	Prompt, PreviousPrompt, Tools, Raw   string
	Diff                                 string
	StartedAt, CompletedAt, FirstTokenAt *int64
	DurationMS                           *int64
	IsError, RequestOnly                 bool
	AssistantID, ParentID                string
	Depth                                int
	Request                              *ledgerRequest
	Usage                                *ledgerUsage
	Seq                                  int
}

// ledgerTurn is one sticky Turn; Group on each record distinguishes Message/Step
// (layout.ts:25-36, 628-642).
type ledgerTurn struct {
	Turn    int
	Title   string
	Records []*ledgerRecord
}

// ledgerModel contains both the grouped rows and the request inspection index.
type ledgerModel struct {
	Turns    []*ledgerTurn
	Records  []*ledgerRecord
	Requests map[string]*ledgerRequest
}

type ledgerBlock struct {
	kind      string
	text      string
	callID    string
	name      string
	arguments string
	raw       json.RawMessage
}

type ledgerStep struct {
	turn, step  int
	key         string
	request     *ledgerRequest
	record      *ledgerRecord
	blocks      []*ledgerBlock
	started     bool
	final       bool
	seen        bool
	interrupted bool
	firstToken  *int64
	usage       *ledgerUsage
	prompt      *ledgerPromptSnapshot
}

type ledgerToolNode struct {
	callID, name     string
	parentID, rootID string
	parent           *ledgerToolNode
	children         []*ledgerToolNode
	record           *ledgerRecord
	started          bool
	settled          bool
	rejected         bool
}

type ledgerCompaction struct {
	id      string
	request *ledgerRequest
	record  *ledgerRecord
	turn    int
	hasTurn bool
}

type ledgerPromptSnapshot struct {
	Config      json.RawMessage
	System      string
	Tools       json.RawMessage
	ToolsByName map[string]json.RawMessage
	SystemSeq   int
	SystemTime  int64
	SystemTurn  int
	SystemStep  int
}

func (p *ledgerPromptSnapshot) clone() *ledgerPromptSnapshot {
	if p == nil {
		return nil
	}
	out := *p
	out.Config = ledgerCloneRaw(p.Config)
	out.Tools = ledgerCloneRaw(p.Tools)
	if p.ToolsByName != nil {
		out.ToolsByName = make(map[string]json.RawMessage, len(p.ToolsByName))
		for name, schema := range p.ToolsByName {
			out.ToolsByName[name] = ledgerCloneRaw(schema)
		}
	}
	return &out
}

type ledgerBaseRecord struct {
	record *ledgerRecord
	seq    int
	order  int
	tool   *ledgerToolNode
}

type ledgerFold struct {
	model          *ledgerModel
	base           []*ledgerBaseRecord
	steps          map[string]*ledgerStep
	stepByRecord   map[string]*ledgerStep
	roots          map[string]*ledgerToolNode
	tools          map[string]*ledgerToolNode
	compactions    map[string]*ledgerCompaction
	currentStep    *ledgerStep
	prompt         *ledgerPromptSnapshot
	hasHeader      bool
	hasSystem      bool
	systemUpdated  bool
	systemRecords  map[int]*ledgerRecord
	nextRequest    int
	nextOrder      int
	lastAssistant  int
	following      []ledgerCoordinate
	inboxPending   []string
	currentClaimed map[string]bool
}

type ledgerCoordinate struct {
	turn, step int
	valid      bool
}

// foldLedger folds raw Session frames into Turn → Message/Step records.
// Lifecycle behavior follows trajectory-assistant-definition.ts:330-419,
// trajectory-tool-definition.ts:123-169, 230-275, and layout.ts:170-554.
func foldLedger(frames []apiclient.Frame) *ledgerModel {
	f := &ledgerFold{
		model:          &ledgerModel{Requests: map[string]*ledgerRequest{}},
		steps:          map[string]*ledgerStep{},
		stepByRecord:   map[string]*ledgerStep{},
		roots:          map[string]*ledgerToolNode{},
		tools:          map[string]*ledgerToolNode{},
		compactions:    map[string]*ledgerCompaction{},
		systemRecords:  map[int]*ledgerRecord{},
		lastAssistant:  0,
		following:      ledgerFollowingCoordinates(frames),
		currentClaimed: map[string]bool{},
	}
	for i, frame := range frames {
		data := ledgerDecodeData(frame.Data)
		switch frame.Type {
		case "system/message":
			f.systemMessage(frame, data)
		case "request/header":
			f.requestHeader(frame, data)
		case "agent/inbox/spliced":
			f.inboxSplice(data)
		case "user/message":
			f.inputMessage(frame, data, false, i)
		case "developer/message":
			f.inputMessage(frame, data, true, i)
		case "step/start":
			f.stepStart(frame, data)
		case "assistant/live-chunk":
			f.liveChunk(frame, data)
		case "assistant/text", "assistant/reasoning":
			f.assistantDelta(frame, data)
		case "assistant/attempt":
			f.assistantAttempt(frame, data)
		case "assistant/message":
			f.assistantMessage(frame, data)
		case "llm/retry":
			f.retry(frame, data)
		case "step/end":
			f.stepEnd(frame, data)
		case "turn/end":
			f.turnEnd(frame, data)
		case "tool/call":
			f.toolCall(frame, data)
		case "tool/result":
			f.toolResult(frame, data)
		case "tool/ptc-dispatch-start", "tool/ptc-dispatch":
			f.ptcDispatch(frame, data)
		case "compaction/start":
			f.compactionStart(frame, data)
		case "compaction/summary":
			f.compactionSummary(frame, data)
		case "compaction/end":
			f.compactionEnd(frame, data)
		case "session/end-seed":
			f.sessionEndSeed(frame)
		}
	}
	return f.finish()
}

// ledgerFollowingCoordinates supplies the input-placement facts used by
// inputPlacement/indexFollowingAssistants (layout.ts:883-918).
func ledgerFollowingCoordinates(frames []apiclient.Frame) []ledgerCoordinate {
	out := make([]ledgerCoordinate, len(frames))
	next := ledgerCoordinate{}
	for i := len(frames) - 1; i >= 0; i-- {
		out[i] = next
		switch frames[i].Type {
		case "step/start", "assistant/live-chunk", "assistant/text", "assistant/reasoning", "assistant/message":
			data := ledgerDecodeData(frames[i].Data)
			turn, step := ledgerInt(data["turn"]), ledgerInt(data["step"])
			if turn > 0 {
				next = ledgerCoordinate{turn: turn, step: step, valid: true}
			}
		}
	}
	return out
}

func (f *ledgerFold) addBase(record *ledgerRecord, seq int, tool *ledgerToolNode) {
	f.nextOrder++
	f.base = append(f.base, &ledgerBaseRecord{record: record, seq: seq, order: f.nextOrder, tool: tool})
}

func ledgerStepKey(turn, step int) string {
	return strconv.Itoa(turn) + "\x00" + strconv.Itoa(step)
}

func ledgerAssistantID(turn, step int) string {
	return "assistant\x00" + strconv.Itoa(turn) + "\x00" + strconv.Itoa(step)
}

func ledgerToolID(kind ledgerKind, callID string) string {
	return string(kind) + "\x00call\x00" + callID
}

func ledgerSeqID(kind ledgerKind, seq int) string {
	return string(kind) + "\x00seq\x00" + strconv.Itoa(seq)
}

func ledgerSetLocation(record *ledgerRecord, turn, step int) {
	if turn < 0 {
		turn = 0
	}
	record.Turn, record.Step = turn, step
	if step > 0 {
		record.Group = "Step " + strconv.Itoa(step)
	} else {
		record.Group = "Message"
	}
}

func (f *ledgerFold) ensureStep(turn, step int) *ledgerStep {
	key := ledgerStepKey(turn, step)
	if existing := f.steps[key]; existing != nil {
		return existing
	}
	id := "assistant:" + strconv.Itoa(turn) + ":" + strconv.Itoa(step)
	f.nextRequest++
	req := &ledgerRequest{
		ID: id, Number: f.nextRequest, Turn: turn, Step: step,
		Status: "running", Purpose: "assistant",
	}
	record := &ledgerRecord{
		ID: ledgerAssistantID(turn, step), Kind: ledgerMessage,
		RequestOnly: true, Request: req, Seq: 0,
	}
	ledgerSetLocation(record, turn, step)
	state := &ledgerStep{turn: turn, step: step, key: key, request: req, record: record}
	f.steps[key] = state
	f.stepByRecord[record.ID] = state
	f.model.Requests[id] = req
	return state
}

// stepStart anchors a request-only row which is filled in place by later output
// (layout.ts:303-325 and trajectory-assistant-definition.ts:347-358).
func (f *ledgerFold) stepStart(frame apiclient.Frame, data map[string]any) {
	turn, step := ledgerInt(data["turn"]), ledgerInt(data["step"])
	state := f.ensureStep(turn, step)
	if !state.started {
		state.started = true
		state.request.StartedAt = ledgerFrameTime(frame)
		state.record.StartedAt = ledgerFrameTime(frame)
		state.record.StartedAt = ledgerFrameTime(frame)
		state.record.Seq = frame.Seq
		state.record.Raw = ledgerRawJSON(frame.Data)
		f.addBase(state.record, frame.Seq, nil)
		if f.hasHeader {
			f.applyPrompt(state, f.prompt)
		}
	}
	f.currentStep = state
}

// systemMessage retains effective prompt replacements and their previous state
// (trajectory-request-header-definition.ts:32-89).
func (f *ledgerFold) systemMessage(frame apiclient.Frame, data map[string]any) {
	message := ledgerObject(data["message"])
	blocks := ledgerRawArray(message["content"])
	text := ledgerContentText(blocks, "text", "\n")
	if f.prompt == nil {
		f.prompt = &ledgerPromptSnapshot{}
	}
	oldText := f.prompt.System
	if f.hasSystem && oldText == text {
		return
	}
	previous := f.prompt.clone()
	f.systemUpdated = f.hasSystem
	f.hasSystem = true
	f.prompt.System = text
	f.prompt.SystemSeq, f.prompt.SystemTime = frame.Seq, frame.Time
	f.prompt.SystemTurn, f.prompt.SystemStep = ledgerInt(data["turn"]), ledgerInt(data["step"])
	kind := "system"
	if !f.hasHeader && previous == nil {
		kind = "initial"
	} else if previous != nil && previous.System == "" && !f.systemUpdated {
		kind = "initial"
	}
	record := &ledgerRecord{
		ID: ledgerSeqID(ledgerSystem, frame.Seq), Kind: ledgerSystem,
		Text: ledgerSystemLabel(kind), Preview: ledgerContentPreview(blocks),
		Input: text, Prompt: ledgerPromptJSON(f.prompt),
		Source: ledgerRawJSON(message["source"]), Raw: ledgerRawJSON(frame.Data),
		StartedAt: ledgerFrameTime(frame), Seq: frame.Seq,
	}
	if previous != nil && (f.hasHeader || previous.System != "") {
		record.PreviousPrompt = ledgerPromptJSON(previous)
		record.Diff = ledgerDiff(previous.System, text)
	}
	turn, step := f.prompt.SystemTurn, f.prompt.SystemStep
	if turn <= 0 {
		turn = 1
	}
	ledgerSetLocation(record, turn, step)
	f.systemRecords[frame.Seq] = record
	f.addBase(record, frame.Seq, nil)
	if state := f.steps[ledgerStepKey(f.prompt.SystemTurn, f.prompt.SystemStep)]; state != nil && state.prompt != nil {
		state.prompt.System = text
		state.prompt.SystemSeq, state.prompt.SystemTime = frame.Seq, frame.Time
		f.applyPrompt(state, state.prompt)
	}
}

// requestHeader applies full config/tools snapshots and emits actual prompt
// changes (request-inspection.ts:93-123; trajectory-snapshot-builder.ts:188-258).
func (f *ledgerFold) requestHeader(frame apiclient.Frame, data map[string]any) {
	header := ledgerObject(data["header"])
	config := ledgerRawMessage(header["config"])
	tools := ledgerRawMessage(header["tools"])
	if len(tools) == 0 || bytes.Equal(bytes.TrimSpace(tools), []byte("null")) {
		tools = json.RawMessage("[]")
	}
	if f.prompt == nil {
		f.prompt = &ledgerPromptSnapshot{}
	}
	previous := f.prompt.clone()
	next := &ledgerPromptSnapshot{
		Config: config, System: f.prompt.System, Tools: tools,
		ToolsByName: ledgerToolSchemas(tools),
		SystemSeq:   f.prompt.SystemSeq, SystemTime: f.prompt.SystemTime,
		SystemTurn: f.prompt.SystemTurn, SystemStep: f.prompt.SystemStep,
	}
	reason, _ := data["reason"].(string)
	initial := !f.hasHeader && reason == "initial"
	systemChanged := f.hasHeader && previous.System != next.System && !f.systemUpdated
	toolsChanged := f.hasHeader && ledgerJSONCompact(previous.Tools) != ledgerJSONCompact(next.Tools)
	change := ""
	switch {
	case initial:
		change = "initial"
	case systemChanged && toolsChanged:
		change = "system-and-tools"
	case systemChanged:
		change = "system"
	case toolsChanged:
		change = "tools"
	}
	f.prompt = next
	f.hasHeader = true
	f.systemUpdated = false
	if f.currentStep != nil {
		f.applyPrompt(f.currentStep, next)
	}
	if change == "" {
		if record := f.systemRecords[next.SystemSeq]; record != nil {
			f.enrichSystemRecord(record, next)
		}
		return
	}
	originSeq, originTime := frame.Seq, frame.Time
	turn, step := 0, 0
	if change == "initial" || systemChanged {
		if next.SystemSeq > 0 {
			originSeq, originTime = next.SystemSeq, next.SystemTime
			turn, step = next.SystemTurn, next.SystemStep
		}
	}
	if record := f.systemRecords[originSeq]; record != nil {
		record.Text = ledgerSystemLabel(change)
		f.enrichSystemRecord(record, next)
		if previous != nil && change != "initial" {
			record.PreviousPrompt = ledgerPromptJSON(previous)
			record.Diff = ledgerDiff(previous.System, next.System)
		}
		return
	}
	record := &ledgerRecord{
		ID: ledgerSeqID(ledgerSystem, originSeq), Kind: ledgerSystem,
		Text: ledgerSystemLabel(change), Preview: next.System,
		Input: next.System, StartedAt: ledgerPositiveTime(originTime), Seq: originSeq,
		Raw: ledgerRawJSON(frame.Data),
	}
	f.enrichSystemRecord(record, next)
	if previous != nil && change != "initial" {
		record.PreviousPrompt = ledgerPromptJSON(previous)
		record.Diff = ledgerDiff(previous.System, next.System)
	}
	if turn <= 0 {
		if f.currentStep != nil {
			turn, step = f.currentStep.turn, f.currentStep.step
		} else {
			turn = 1
		}
	}
	ledgerSetLocation(record, turn, step)
	f.systemRecords[originSeq] = record
	f.addBase(record, originSeq, nil)
}

func (f *ledgerFold) enrichSystemRecord(record *ledgerRecord, prompt *ledgerPromptSnapshot) {
	record.Prompt = ledgerPromptJSON(prompt)
	record.Options = ledgerRawJSON(prompt.Config)
	record.Tools = ledgerRawJSON(prompt.Tools)
	if record.Source == "" {
		record.Source = ledgerRawJSON(prompt.Tools)
	}
}

func ledgerSystemLabel(change string) string {
	switch change {
	case "initial":
		return "Initial system prompt"
	case "tools":
		return "Tools updated"
	case "system-and-tools":
		return "System prompt and tools updated"
	default:
		return "System prompt updated"
	}
}

func (f *ledgerFold) applyPrompt(state *ledgerStep, prompt *ledgerPromptSnapshot) {
	if state == nil || prompt == nil {
		return
	}
	state.prompt = prompt.clone()
	state.record.Prompt = ledgerPromptJSON(prompt)
	state.record.Options = ledgerRawJSON(prompt.Config)
	state.record.Tools = ledgerRawJSON(prompt.Tools)
	state.request.Options = ledgerRawJSON(prompt.Config)
	provider, model := ledgerConfigRoute(prompt.Config)
	if provider != "" {
		state.request.Provider = provider
	}
	if model != "" {
		state.request.Model = model
	}
}

// inboxSplice applies the durable next-step queue and current-claim transition
// (trajectory-message-definitions.ts:78-134). Pending splices are materialized
// eagerly; a canceled claim changes the queue but never claims removed IDs.
func (f *ledgerFold) inboxSplice(data map[string]any) {
	if ledgerString(data["target"]) != "next-step" {
		return
	}
	start, removedCount := ledgerInt(data["start"]), ledgerInt(data["removedCount"])
	if start < 0 || removedCount < 0 || start > len(f.inboxPending) || start+removedCount > len(f.inboxPending) {
		return
	}
	var inserted []string
	for _, value := range ledgerAnyArray(data["inserted"]) {
		if id := ledgerString(ledgerObject(value)["id"]); id != "" {
			inserted = append(inserted, id)
		}
	}
	removed := append([]string(nil), f.inboxPending[start:start+removedCount]...)
	pending := make([]string, 0, len(f.inboxPending)-removedCount+len(inserted))
	pending = append(pending, f.inboxPending[:start]...)
	pending = append(pending, inserted...)
	pending = append(pending, f.inboxPending[start+removedCount:]...)
	f.inboxPending = pending
	if removedCount > 0 && ledgerString(data["outcome"]) != "canceled" {
		f.currentClaimed = make(map[string]bool, len(removed))
		for _, id := range removed {
			f.currentClaimed[id] = true
		}
		return
	}
	for _, id := range inserted {
		delete(f.currentClaimed, id)
	}
}

// inputMessage classifies user-role sources exactly like
// trajectory-message-definitions.ts:136-195.
func (f *ledgerFold) inputMessage(frame apiclient.Frame, data map[string]any, developer bool, index int) {
	message := data
	if developer {
		message = ledgerObject(data["message"])
	}
	source := ledgerObject(message["source"])
	kind := ledgerContext
	if !developer && ledgerString(source["kind"]) == "user" {
		kind = ledgerUser
	}
	steering := kind == ledgerUser && f.currentClaimed[ledgerString(message["id"])]
	blocks := ledgerRawArray(message["content"])
	input := ledgerContentText(blocks, "text", "\n")
	if input == "" && len(blocks) > 0 {
		input = ledgerRawJSON(message["content"])
	}
	record := &ledgerRecord{
		ID: ledgerSeqID(kind, frame.Seq), Kind: kind,
		Preview: ledgerContentPreview(blocks), Input: input,
		Source: ledgerRawJSON(message["source"]), Raw: ledgerRawJSON(frame.Data),
		StartedAt: ledgerFrameTime(frame), Seq: frame.Seq,
	}
	turn, step := ledgerInt(data["turn"]), ledgerInt(data["step"])
	coordinate := f.following[index]
	if turn <= 0 && coordinate.valid {
		turn, step = coordinate.turn, coordinate.step
	} else if step <= 0 && coordinate.valid && coordinate.turn == turn {
		step = coordinate.step
	}
	if turn <= 0 {
		if steering {
			// Without a following request location, steering remains in the preceding turn.
			turn = f.lastAssistant
		} else {
			turn = f.lastAssistant + 1
		}
		if turn < 1 {
			turn = 1
		}
	}
	ledgerSetLocation(record, turn, step)
	f.addBase(record, frame.Seq, nil)
}

func (f *ledgerFold) liveChunk(frame apiclient.Frame, data map[string]any) {
	state := f.ensureStep(ledgerInt(data["turn"]), ledgerInt(data["step"]))
	state.seen, state.record.RequestOnly = true, false
	state.record.Raw = ledgerRawJSON(frame.Data)
	chunk := ledgerObject(data["chunk"])
	ledgerApplyChunk(state, chunk, frame.Time)
}

func (f *ledgerFold) assistantDelta(frame apiclient.Frame, data map[string]any) {
	state := f.ensureStep(ledgerInt(data["turn"]), ledgerInt(data["step"]))
	state.seen, state.record.RequestOnly = true, false
	state.record.Raw = ledgerRawJSON(frame.Data)
	kind := "text"
	if frame.Type == "assistant/reasoning" {
		kind = "reasoning"
	}
	chunk := map[string]any{
		"type": kind + "-delta", "index": ledgerInt(data["index"]),
		"text": ledgerString(data["text"]),
	}
	ledgerApplyChunk(state, chunk, frame.Time)
}

// ledgerApplyChunk retains stream block indexes and first non-empty token time
// (trajectory-assistant-definition.ts:116-190; trajectory-event-projection.ts:140-155).
func ledgerApplyChunk(state *ledgerStep, chunk map[string]any, eventTime int64) {
	typ := ledgerString(chunk["type"])
	index := ledgerInt(chunk["index"])
	switch typ {
	case "block-start":
		ledgerSetBlock(state, index, &ledgerBlock{kind: ledgerString(chunk["blockType"])})
	case "text-delta", "reasoning-delta":
		kind := strings.TrimSuffix(typ, "-delta")
		block := ledgerBlockAt(state, index, kind)
		block.text += ledgerString(chunk["text"])
		if block.text != "" && state.firstToken == nil && ledgerString(chunk["text"]) != "" {
			state.firstToken = ledgerPositiveTime(eventTime)
		}
	case "tool-call-delta":
		block := ledgerBlockAt(state, index, "tool-call")
		if block.callID == "" {
			block.callID = ledgerString(chunk["id"])
		}
		if name := ledgerString(chunk["name"]); name != "" {
			block.name = name
		}
		delta := ledgerString(chunk["argumentsDelta"])
		block.arguments += delta
		if state.firstToken == nil && (delta != "" || ledgerString(chunk["name"]) != "") {
			state.firstToken = ledgerPositiveTime(eventTime)
		}
	case "block-end":
		if block := ledgerParseBlock(ledgerRawMessage(chunk["block"])); block != nil {
			ledgerSetBlock(state, index, block)
		}
	case "usage":
		state.usage = ledgerAddUsage(state.usage, ledgerParseUsage(ledgerRawMessage(chunk["usage"])))
	}
}

func ledgerBlockAt(state *ledgerStep, index int, kind string) *ledgerBlock {
	if index < 0 {
		index = 0
	}
	for len(state.blocks) <= index {
		state.blocks = append(state.blocks, nil)
	}
	if state.blocks[index] == nil || state.blocks[index].kind != kind {
		state.blocks[index] = &ledgerBlock{kind: kind}
	}
	return state.blocks[index]
}

func ledgerSetBlock(state *ledgerStep, index int, block *ledgerBlock) {
	if index < 0 {
		index = 0
	}
	for len(state.blocks) <= index {
		state.blocks = append(state.blocks, nil)
	}
	state.blocks[index] = block
}

// assistantAttempt applies settleTiming without manufacturing a surface message
// (trajectory-assistant-definition.ts:193-201, 231-245, 359-365).
func (f *ledgerFold) assistantAttempt(frame apiclient.Frame, data map[string]any) {
	state := f.ensureStep(ledgerInt(data["turn"]), ledgerInt(data["step"]))
	if state.firstToken == nil {
		state.firstToken = ledgerStreamFirstToken(ledgerRawArray(data["stream"]))
	}
	if !state.seen {
		state.record.Raw = ledgerRawJSON(frame.Data)
	}
}

// assistantMessage settles authoritative blocks, route, usage, and stream timing
// (trajectory-assistant-definition.ts:193-217, 251-327).
func (f *ledgerFold) assistantMessage(frame apiclient.Frame, data map[string]any) {
	turn, step := ledgerInt(data["turn"]), ledgerInt(data["step"])
	state := f.ensureStep(turn, step)
	state.seen, state.final, state.record.RequestOnly = true, true, false
	state.interrupted = ledgerBool(data["interrupted"])
	state.blocks = nil
	message := ledgerObject(data["message"])
	for i, raw := range ledgerRawArray(message["content"]) {
		if block := ledgerParseBlock(raw); block != nil {
			ledgerSetBlock(state, i, block)
		}
	}
	state.usage = ledgerParseUsage(ledgerRawMessage(data["usage"]))
	if state.firstToken == nil {
		state.firstToken = ledgerStreamFirstToken(ledgerRawArray(data["stream"]))
	}
	state.request.CompletedAt = ledgerFrameTime(frame)
	state.request.ResultSeq = frame.Seq
	state.request.Status = "complete"
	state.record.CompletedAt = ledgerFrameTime(frame)
	state.record.Seq = frame.Seq
	state.record.Raw = ledgerRawJSON(frame.Data)
	state.record.Source = ledgerRawJSON(message["content"])
	state.record.IsError = state.interrupted
	if state.interrupted {
		state.request.Status = "error"
	}
	source := ledgerObject(message["source"])
	if provider := ledgerString(source["provider"]); provider != "" {
		state.request.Provider = provider
	}
	if model := ledgerString(source["model"]); model != "" {
		state.request.Model = model
	}
	f.lastAssistant = turn
	f.currentStep = state
}

// retry preserves first-token/usage facts while resetting the visible attempt
// (trajectory-assistant-definition.ts:359-386).
func (f *ledgerFold) retry(frame apiclient.Frame, data map[string]any) {
	state := f.ensureStep(ledgerInt(data["turn"]), ledgerInt(data["step"]))
	message, code := ledgerDisplayFailure(ledgerRawMessage(data["failure"]))
	if len(ledgerRawMessage(data["failure"])) == 0 {
		message, code = ledgerDisplayFailure(ledgerRawMessage(data["error"]))
	}
	state.request.Error, state.request.ErrorCode = message, code
	state.request.Retry = ledgerInt64Pointer(data["retry"])
	if state.request.Retry == nil {
		state.request.Retry = ledgerInt64Pointer(data["attempt"])
	}
	if ledgerString(data["mode"]) == "normal" {
		state.request.MaxRetries = ledgerInt64Pointer(data["maxRetries"])
	} else {
		state.request.MaxRetries = nil
	}
	state.request.RetryDelayMS = ledgerInt64Pointer(data["delayMs"])
	state.request.Status = "running"
	state.blocks = nil
	state.final = false
	state.interrupted = false
	state.record.RequestOnly = !state.seen
	state.record.CompletedAt = nil
	state.record.DurationMS = nil
	state.request.CompletedAt = nil
	state.request.ResultSeq = 0
}

func (f *ledgerFold) stepEnd(frame apiclient.Frame, data map[string]any) {
	state := f.steps[ledgerStepKey(ledgerInt(data["turn"]), ledgerInt(data["step"]))]
	if state == nil {
		return
	}
	if state.request.CompletedAt == nil {
		state.request.CompletedAt = ledgerFrameTime(frame)
		state.record.CompletedAt = ledgerFrameTime(frame)
	}
	if !state.final {
		state.request.Status = "error"
		state.record.IsError = true
	}
	if f.currentStep == state {
		f.currentStep = nil
	}
}

// turnEnd applies only the recorded structured failure; AUTH messages are
// removed before entering UI state (trajectory-snapshot-builder.ts:126-147).
func (f *ledgerFold) turnEnd(frame apiclient.Frame, data map[string]any) {
	turn := ledgerInt(data["turn"])
	reason := ledgerObject(data["reason"])
	if ledgerString(reason["kind"]) != "error" {
		return
	}
	var latest *ledgerStep
	for _, state := range f.steps {
		if state.turn == turn && (latest == nil || state.request.Number > latest.request.Number) {
			latest = state
		}
	}
	if latest == nil {
		return
	}
	message, code := ledgerDisplayFailure(ledgerRawMessage(reason["error"]))
	latest.request.Status = "error"
	latest.request.Error, latest.request.ErrorCode = message, code
	latest.record.IsError = true
	if latest.request.CompletedAt == nil {
		latest.request.CompletedAt = ledgerFrameTime(frame)
		latest.record.CompletedAt = ledgerFrameTime(frame)
	}
}

// toolCall and toolResult implement the root-call lifecycle in
// trajectory-tool-definition.ts:35-73, 230-275.
func (f *ledgerFold) toolCall(frame apiclient.Frame, data map[string]any) {
	callID := ledgerString(data["callId"])
	if callID == "" {
		return
	}
	turn, step := ledgerInt(data["turn"]), ledgerInt(data["step"])
	tool := f.ensureRoot(callID, turn, step, true, frame.Seq)
	tool.name = ledgerString(data["name"])
	tool.started = true
	tool.record.Text = tool.name
	tool.record.Input = ledgerRawString(ledgerRawMessage(data["arguments"]))
	tool.record.Preview = tool.record.Input
	tool.record.StartedAt = ledgerFrameTime(frame)
	tool.record.Seq = frame.Seq
	tool.record.Raw = ledgerRawJSON(frame.Data)
	tool.record.Schema = f.schemaFor(tool.name)
	if turn > 0 && step > 0 {
		tool.record.AssistantID = ledgerAssistantID(turn, step)
		tool.record.ParentID = tool.record.AssistantID
	}
}

func (f *ledgerFold) toolResult(frame apiclient.Frame, data map[string]any) {
	message := ledgerObject(data["message"])
	source := ledgerObject(message["source"])
	callID := ledgerString(source["callId"])
	if callID == "" {
		callID = ledgerString(message["toolCallId"])
	}
	if callID == "" {
		callID = ledgerString(data["callId"])
	}
	if callID == "" {
		return
	}
	turn, step := ledgerInt(data["turn"]), ledgerInt(data["step"])
	tool := f.ensureRoot(callID, turn, step, true, frame.Seq)
	tool.settled = true
	tool.record.CompletedAt = ledgerFrameTime(frame)
	tool.record.IsError = ledgerBool(message["isError"])
	blocks := ledgerRawArray(message["content"])
	tool.record.Output = ledgerToolOutput(blocks, tool.record.IsError, ledgerRawMessage(data["error"]))
	tool.record.Source = ledgerRawJSON(message["content"])
	tool.record.Raw = ledgerRawJSON(frame.Data)
	if tool.record.Text == "" {
		tool.record.Text = tool.record.Output
	}
}

func (f *ledgerFold) ensureRoot(callID string, turn, step int, addBase bool, seq int) *ledgerToolNode {
	if existing := f.roots[callID]; existing != nil {
		if existing.record.Turn <= 0 && turn > 0 {
			ledgerSetLocation(existing.record, turn, step)
		}
		return existing
	}
	if turn <= 0 {
		if f.currentStep != nil {
			turn, step = f.currentStep.turn, f.currentStep.step
		} else {
			turn = 1
		}
	}
	record := &ledgerRecord{
		ID: ledgerToolID(ledgerTool, callID), Kind: ledgerTool, Depth: 1, Seq: seq,
	}
	if turn > 0 && step > 0 {
		record.AssistantID = ledgerAssistantID(turn, step)
		record.ParentID = record.AssistantID
	}
	ledgerSetLocation(record, turn, step)
	tool := &ledgerToolNode{callID: callID, record: record}
	f.roots[callID], f.tools[callID] = tool, tool
	if addBase {
		f.addBase(record, seq, tool)
	}
	return tool
}

// ptcDispatch retains every real sub-call even when its parent is late or absent.
// Edges are resolved only after all frames arrive, so no empty synthetic root is needed
// (trajectory-tool-definition.ts:85-169, 181-214).
func (f *ledgerFold) ptcDispatch(frame apiclient.Frame, data map[string]any) {
	parentID, childID := ledgerString(data["parentCallId"]), ledgerString(data["subCallId"])
	if parentID == "" || childID == "" {
		return
	}
	rootID := ledgerString(data["rootCallId"])
	if rootID == "" {
		rootID = parentID
	}
	child := f.tools[childID]
	if child != nil && child.record.Kind != ledgerSubtool {
		// A real native root cannot also become its own PTC descendant.
		return
	}
	isStart := frame.Type == "tool/ptc-dispatch-start"
	if child != nil {
		if child.parentID != parentID || child.rootID != rootID || (isStart && (child.started || child.settled)) {
			return
		}
	} else {
		parentKind := ledgerSubtool
		if parentID == rootID {
			parentKind = ledgerTool
		}
		record := &ledgerRecord{
			ID: ledgerToolID(ledgerSubtool, childID), Kind: ledgerSubtool,
			ParentID: ledgerToolID(parentKind, parentID), Seq: frame.Seq,
		}
		if parent := f.tools[parentID]; parent != nil {
			record.AssistantID = parent.record.AssistantID
			ledgerSetLocation(record, parent.record.Turn, parent.record.Step)
		}
		if f.currentStep != nil {
			if record.AssistantID == "" {
				record.AssistantID = f.currentStep.record.ID
			}
			if record.Turn <= 0 {
				ledgerSetLocation(record, f.currentStep.turn, f.currentStep.step)
			}
		}
		if record.Turn <= 0 {
			ledgerSetLocation(record, 1, 0)
		}
		child = &ledgerToolNode{callID: childID, parentID: parentID, rootID: rootID, record: record}
		f.tools[childID] = child
		// The base row is skipped at finish when a real parent adopts this child.
		f.addBase(record, frame.Seq, child)
	}
	child.name = ledgerString(data["name"])
	child.record.Text = child.name
	child.record.Input = ledgerRawJSON(data["arguments"])
	child.record.Preview = child.record.Input
	child.record.Raw = ledgerRawJSON(frame.Data)
	child.record.Schema = f.schemaFor(child.name)
	if isStart {
		child.started = true
		child.record.StartedAt = ledgerFrameTime(frame)
		return
	}
	child.settled = true
	child.record.CompletedAt = ledgerFrameTime(frame)
	child.record.IsError = ledgerBool(data["isError"])
	blocks := ledgerRawArray(data["content"])
	child.record.Output = ledgerToolOutput(blocks, child.record.IsError, ledgerRawMessage(data["error"]))
	child.record.Source = ledgerRawJSON(data["content"])
}

type ledgerPTCDepth struct {
	depth int
	known bool
	valid bool
}

// linkPTC resolves late parent edges, rejects cycles/depth overflow, and propagates
// the real assistant coordinate through each accepted tree.
func (f *ledgerFold) linkPTC() {
	for _, node := range f.tools {
		node.parent = nil
		node.children = nil
		node.rejected = false
	}
	memo := map[*ledgerToolNode]ledgerPTCDepth{}
	visiting := map[*ledgerToolNode]bool{}
	var resolve func(*ledgerToolNode) ledgerPTCDepth
	resolve = func(node *ledgerToolNode) ledgerPTCDepth {
		if node == nil {
			return ledgerPTCDepth{}
		}
		if node.record.Kind == ledgerTool {
			return ledgerPTCDepth{depth: 1, known: true, valid: true}
		}
		if cached, ok := memo[node]; ok {
			return cached
		}
		if visiting[node] {
			return ledgerPTCDepth{}
		}
		visiting[node] = true
		parent := f.tools[node.parentID]
		result := ledgerPTCDepth{valid: true}
		switch {
		case parent == nil && node.parentID == node.rootID:
			// The durable root ID establishes depth even when the root row is outside history.
			result.depth, result.known = 2, true
		case parent == nil:
			// A missing non-root ancestor leaves depth unknown; the real child is still retained.
			result.depth, result.known = 0, false
		case parent.record.Kind == ledgerTool && node.parentID != node.rootID:
			result.valid = false
		default:
			parentDepth := resolve(parent)
			switch {
			case !parentDepth.valid:
				result.valid = false
			case parentDepth.known && parentDepth.depth >= 256:
				result.valid = false
			case parentDepth.known:
				result.depth, result.known = parentDepth.depth+1, true
			}
		}
		delete(visiting, node)
		memo[node] = result
		return result
	}
	for _, node := range f.tools {
		if node.record.Kind != ledgerSubtool {
			continue
		}
		result := resolve(node)
		node.rejected = !result.valid
		if result.valid && result.known {
			node.record.Depth = result.depth
		} else {
			node.record.Depth = 0
		}
	}
	for _, node := range f.tools {
		if node.record.Kind != ledgerSubtool || node.rejected {
			continue
		}
		if parent := f.tools[node.parentID]; parent != nil && !parent.rejected {
			node.parent = parent
			parent.children = append(parent.children, node)
		}
	}
	var inherit func(*ledgerToolNode)
	inherit = func(parent *ledgerToolNode) {
		sort.SliceStable(parent.children, func(i, j int) bool {
			if parent.children[i].record.Seq != parent.children[j].record.Seq {
				return parent.children[i].record.Seq < parent.children[j].record.Seq
			}
			return parent.children[i].callID < parent.children[j].callID
		})
		for _, child := range parent.children {
			if parent.record.AssistantID != "" {
				child.record.AssistantID = parent.record.AssistantID
			}
			if parent.record.Turn > 0 {
				ledgerSetLocation(child.record, parent.record.Turn, parent.record.Step)
			}
			inherit(child)
		}
	}
	for _, node := range f.tools {
		if node.parent == nil && !node.rejected {
			inherit(node)
		}
	}
}

func (f *ledgerFold) schemaFor(name string) string {
	if f.prompt == nil || name == "" {
		return ""
	}
	return ledgerRawJSON(f.prompt.ToolsByName[name])
}

// compactionStart/Summary/End preserve the one-shot request facts in
// trajectory-compaction-definition.ts:40-110.
func (f *ledgerFold) compactionStart(frame apiclient.Frame, data map[string]any) {
	id := ledgerString(data["compactionId"])
	if id == "" {
		id = "seq:" + strconv.Itoa(frame.Seq)
	}
	f.nextRequest++
	turn := ledgerInt(data["turn"])
	_, hasTurn := data["turn"]
	if data["turn"] == nil {
		hasTurn = false
	}
	requestID := "compaction:" + strconv.Itoa(frame.Seq)
	req := &ledgerRequest{
		ID: requestID, Number: f.nextRequest, Turn: turn, Status: "running",
		Purpose: "compaction", StartedAt: ledgerFrameTime(frame), Compaction: true,
	}
	record := &ledgerRecord{
		ID: ledgerSeqID(ledgerCompacted, frame.Seq), Kind: ledgerCompacted,
		Text: "Compacting", Turn: turn, Group: "Compaction " + strconv.Itoa(frame.Seq),
		StartedAt: ledgerFrameTime(frame), Request: req, Seq: frame.Seq,
		Raw: ledgerRawJSON(frame.Data),
	}
	state := &ledgerCompaction{id: id, request: req, record: record, turn: turn, hasTurn: hasTurn}
	f.compactions[id] = state
	f.model.Requests[requestID] = req
	f.addBase(record, frame.Seq, nil)
}

func (f *ledgerFold) compactionSummary(frame apiclient.Frame, data map[string]any) {
	state := f.compactions[ledgerString(data["compactionId"])]
	if state == nil {
		return
	}
	summary := ledgerRawArray(data["summary"])
	rawOutput := ledgerRawArray(data["rawOutput"])
	source := rawOutput
	if len(source) == 0 {
		source = summary
	}
	state.record.Output = ledgerContentText(summary, "text", "\n")
	state.record.Preview = ledgerContentPreview(summary)
	state.record.Thinking = ledgerContentText(source, "reasoning", "\n")
	state.record.Source = ledgerRawArrayJSON(source)
	state.record.Raw = ledgerRawArrayJSON(source)
	state.record.StartedAt = state.request.StartedAt
	state.request.ResultSeq = frame.Seq
	state.request.Provider = ledgerString(data["provider"])
	state.request.Model = ledgerString(data["model"])
	state.request.Usage = ledgerParseUsage(ledgerRawMessage(data["usage"]))
	state.record.Usage = state.request.Usage
	options := map[string]any{
		"provider": ledgerString(data["provider"]), "model": ledgerString(data["model"]),
		"purpose": "compaction",
	}
	if maxTokens, ok := data["maxTokens"]; ok {
		options["maxTokens"] = maxTokens
	}
	state.request.Options = ledgerRawJSON(options)
	state.record.Options = state.request.Options
}

func (f *ledgerFold) compactionEnd(frame apiclient.Frame, data map[string]any) {
	state := f.compactions[ledgerString(data["compactionId"])]
	if state == nil {
		return
	}
	state.request.CompletedAt = ledgerFrameTime(frame)
	state.record.CompletedAt = ledgerFrameTime(frame)
	state.request.Status = "complete"
	state.record.Text = "Compacted"
	if raw := ledgerRawMessage(data["error"]); len(raw) > 0 && !bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		state.request.Status = "error"
		state.request.Error = ledgerString(data["error"])
		state.record.Text = state.request.Error
		state.record.IsError = true
	}
}

const ledgerCompactionInterrupted = "trajectory.compaction-interrupted"

// sessionEndSeed closes the latest unmatched compaction bracket before the
// boundary (trajectory-snapshot-builder.ts:95-123; copy-codes.ts:1-4).
func (f *ledgerFold) sessionEndSeed(frame apiclient.Frame) {
	var latest *ledgerCompaction
	for _, state := range f.compactions {
		if state.request.Status != "running" || state.record.Seq >= frame.Seq {
			continue
		}
		if latest == nil || state.record.Seq > latest.record.Seq {
			latest = state
		}
	}
	if latest == nil {
		return
	}
	latest.request.Status = "error"
	latest.request.Error = ledgerCompactionInterrupted
	latest.request.ErrorCode = ledgerCompactionInterrupted
	latest.request.CompletedAt = ledgerFrameTime(frame)
	latest.record.IsError = true
	latest.record.Text = ledgerCompactionInterrupted
	latest.record.CompletedAt = ledgerFrameTime(frame)
}

// finish expands assistant tool blocks and recursive sub-calls immediately after
// their owner (layout.ts:706-801, 990-1057), then assigns dense display indexes.
func (f *ledgerFold) finish() *ledgerModel {
	for _, state := range f.steps {
		f.refreshAssistant(state)
	}
	attachedOwner := map[string]*ledgerStep{}
	attachedOrder := map[string][]*ledgerToolNode{}
	for _, state := range f.steps {
		for _, block := range state.blocks {
			if block == nil || block.kind != "tool-call" || block.callID == "" {
				continue
			}
			if attachedOwner[block.callID] != nil {
				continue
			}
			tool := f.ensureRoot(block.callID, state.turn, state.step, false, state.record.Seq)
			attachedOwner[block.callID] = state
			tool.record.AssistantID = state.record.ID
			tool.record.ParentID = state.record.ID
			if block.name != "" {
				tool.name, tool.record.Text = block.name, block.name
			}
			tool.record.Input = block.arguments
			tool.record.Preview = block.arguments
			if tool.record.Raw == "" {
				tool.record.Raw = ledgerRawJSON(block.raw)
			}
			if tool.record.Schema == "" && state.prompt != nil {
				tool.record.Schema = ledgerRawJSON(state.prompt.ToolsByName[tool.name])
			}
			attachedOrder[state.record.ID] = append(attachedOrder[state.record.ID], tool)
		}
	}
	f.linkPTC()
	sort.SliceStable(f.base, func(i, j int) bool {
		if f.base[i].seq != f.base[j].seq {
			return f.base[i].seq < f.base[j].seq
		}
		return f.base[i].order < f.base[j].order
	})
	records := make([]*ledgerRecord, 0, len(f.base)+len(f.tools))
	emitted := map[string]bool{}
	var appendRecord func(*ledgerRecord)
	appendRecord = func(record *ledgerRecord) {
		if record == nil || emitted[record.ID] {
			return
		}
		emitted[record.ID] = true
		records = append(records, record)
		if state := f.stepByRecord[record.ID]; state != nil {
			for _, tool := range attachedOrder[record.ID] {
				f.appendToolTree(&records, emitted, tool, map[string]bool{})
			}
		}
	}
	for _, entry := range f.base {
		if entry.tool != nil {
			if entry.tool.rejected || entry.tool.parent != nil || attachedOwner[entry.tool.callID] != nil {
				continue
			}
			f.appendToolTreeWithRecord(&records, emitted, entry.tool, map[string]bool{}, appendRecord)
			continue
		}
		appendRecord(entry.record)
	}
	// A block-only root has no tool/call base entry, but remains attached above.
	for _, state := range f.steps {
		if !emitted[state.record.ID] {
			appendRecord(state.record)
		}
	}
	f.model.Records = records
	turnAt := map[int]int{}
	for i, record := range records {
		record.Index = i + 1
		at, ok := turnAt[record.Turn]
		if !ok {
			title := "Turn " + strconv.Itoa(record.Turn)
			if record.Turn == 0 {
				title = "Between turns"
			}
			f.model.Turns = append(f.model.Turns, &ledgerTurn{Turn: record.Turn, Title: title})
			at = len(f.model.Turns) - 1
			turnAt[record.Turn] = at
		}
		f.model.Turns[at].Records = append(f.model.Turns[at].Records, record)
	}
	f.assignCumulativeUsage()
	return f.model
}

func (f *ledgerFold) appendToolTree(records *[]*ledgerRecord, emitted map[string]bool, tool *ledgerToolNode, visiting map[string]bool) {
	f.appendToolTreeWithRecord(records, emitted, tool, visiting, func(record *ledgerRecord) {
		if record == nil || emitted[record.ID] {
			return
		}
		emitted[record.ID] = true
		*records = append(*records, record)
	})
}

func (f *ledgerFold) appendToolTreeWithRecord(records *[]*ledgerRecord, emitted map[string]bool, tool *ledgerToolNode, visiting map[string]bool, appendRecord func(*ledgerRecord)) {
	if tool == nil || tool.rejected || visiting[tool.callID] || tool.record.Depth > 256 {
		return
	}
	visiting[tool.callID] = true
	appendRecord(tool.record)
	for _, child := range tool.children {
		f.appendToolTree(records, emitted, child, visiting)
	}
	delete(visiting, tool.callID)
}

func (f *ledgerFold) refreshAssistant(state *ledgerStep) {
	record := state.record
	var texts, reasoning []string
	for _, block := range state.blocks {
		if block == nil {
			continue
		}
		switch block.kind {
		case "text":
			texts = append(texts, block.text)
		case "reasoning":
			reasoning = append(reasoning, block.text)
		}
	}
	record.Output = strings.Join(texts, "\n\n")
	record.Thinking = strings.Join(reasoning, "\n\n")
	record.Preview = record.Output
	if record.Preview == "" {
		record.Preview = record.Thinking
	}
	if record.Output == "" && record.Thinking == "" {
		for _, block := range state.blocks {
			if block != nil && block.kind == "tool-call" {
				record.Text = "Tool call"
				break
			}
		}
	}
	record.FirstTokenAt = state.firstToken
	record.Usage = state.usage
	state.request.Usage = state.usage
	if !state.final && state.seen {
		blocks := make([]json.RawMessage, 0, len(state.blocks))
		for _, block := range state.blocks {
			if block == nil {
				continue
			}
			if len(block.raw) > 0 {
				blocks = append(blocks, block.raw)
				continue
			}
			switch block.kind {
			case "text", "reasoning":
				raw, _ := json.Marshal(map[string]any{"type": block.kind, "text": block.text})
				blocks = append(blocks, raw)
			case "tool-call":
				raw, _ := json.Marshal(map[string]any{
					"type": "tool-call", "id": block.callID, "name": block.name,
					"arguments": block.arguments,
				})
				blocks = append(blocks, raw)
			}
		}
		record.Source = ledgerRawArrayJSON(blocks)
	}
	record.DurationMS = ledgerDuration(record.StartedAt, record.CompletedAt)
}

func (f *ledgerFold) assignCumulativeUsage() {
	requests := make([]*ledgerRequest, 0, len(f.model.Requests))
	for _, request := range f.model.Requests {
		requests = append(requests, request)
	}
	sort.Slice(requests, func(i, j int) bool { return requests[i].Number < requests[j].Number })
	var cumulative *ledgerUsage
	for _, request := range requests {
		cumulative = ledgerAddUsage(cumulative, request.Usage)
		request.CumulativeUsage = ledgerCloneUsage(cumulative)
	}
	for _, tool := range f.tools {
		tool.record.DurationMS = ledgerDuration(tool.record.StartedAt, tool.record.CompletedAt)
	}
	for _, compaction := range f.compactions {
		compaction.record.DurationMS = ledgerDuration(compaction.record.StartedAt, compaction.record.CompletedAt)
	}
}

// ledgerUsageJSON serializes only buckets actually supplied by the provider.
func ledgerUsageJSON(usage *ledgerUsage) string {
	if usage == nil {
		return ""
	}
	wire := struct {
		Input      *int64 `json:"inputTokens,omitempty"`
		CacheRead  *int64 `json:"cacheReadTokens,omitempty"`
		CacheWrite *int64 `json:"cacheWriteTokens,omitempty"`
		Output     *int64 `json:"outputTokens,omitempty"`
		Reasoning  *int64 `json:"reasoningTokens,omitempty"`
	}{usage.Input, usage.CacheRead, usage.CacheWrite, usage.Output, usage.Reasoning}
	raw, err := json.Marshal(wire)
	if err != nil {
		return ""
	}
	return string(raw)
}

// ledgerSearchText mirrors the Inspector fields indexed by
// trajectory-search-index.ts:39-72 and never indexes request-only separators.
func ledgerSearchText(record *ledgerRecord) string {
	if record == nil || record.RequestOnly {
		return ""
	}
	parts := []string{
		record.ID, strconv.Itoa(record.Turn), strconv.Itoa(record.Step), record.Group,
		string(record.Kind), record.Text, record.Preview, record.Input, record.Output,
		record.Thinking, record.Source, record.Schema, record.Options, record.Prompt,
		record.PreviousPrompt, record.Tools, record.Raw, record.Diff,
		record.AssistantID, record.ParentID, ledgerUsageJSON(record.Usage),
	}
	if record.Kind == ledgerMessage {
		parts = append(parts, "assistant")
	}
	if request := record.Request; request != nil {
		parts = append(parts, request.ID, request.Status, request.Purpose, request.Provider,
			request.Model, request.Error, request.ErrorCode, request.Options,
			ledgerUsageJSON(request.Usage), ledgerUsageJSON(request.CumulativeUsage))
	}
	return strings.ToLower(strings.Join(parts, "\n"))
}

// ledgerDiff returns a real line-based unified-ish replacement hunk. Common
// prefix/suffix lines are retained as three lines of context; no content is invented.
func ledgerDiff(before, after string) string {
	if before == after {
		return ""
	}
	oldLines, newLines := strings.Split(before, "\n"), strings.Split(after, "\n")
	if before == "" {
		oldLines = nil
	}
	if after == "" {
		newLines = nil
	}
	prefix := 0
	for prefix < len(oldLines) && prefix < len(newLines) && oldLines[prefix] == newLines[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(oldLines)-prefix && suffix < len(newLines)-prefix &&
		oldLines[len(oldLines)-1-suffix] == newLines[len(newLines)-1-suffix] {
		suffix++
	}
	contextBefore := prefix
	if contextBefore > 3 {
		contextBefore = 3
	}
	contextAfter := suffix
	if contextAfter > 3 {
		contextAfter = 3
	}
	oldStart := prefix - contextBefore + 1
	newStart := oldStart
	oldCount := contextBefore + len(oldLines) - prefix - suffix + contextAfter
	newCount := contextBefore + len(newLines) - prefix - suffix + contextAfter
	if len(oldLines) == 0 {
		oldStart, oldCount = 0, 0
	}
	if len(newLines) == 0 {
		newStart, newCount = 0, 0
	}
	var out strings.Builder
	out.WriteString("--- before\n+++ after\n")
	fmt.Fprintf(&out, "@@ -%d,%d +%d,%d @@\n", oldStart, oldCount, newStart, newCount)
	for i := prefix - contextBefore; i < prefix; i++ {
		out.WriteString(" " + oldLines[i] + "\n")
	}
	for i := prefix; i < len(oldLines)-suffix; i++ {
		out.WriteString("-" + oldLines[i] + "\n")
	}
	for i := prefix; i < len(newLines)-suffix; i++ {
		out.WriteString("+" + newLines[i] + "\n")
	}
	for i := len(newLines) - suffix; i < len(newLines)-suffix+contextAfter; i++ {
		out.WriteString(" " + newLines[i] + "\n")
	}
	return out.String()
}

func ledgerParseBlock(raw json.RawMessage) *ledgerBlock {
	object := ledgerObjectRaw(raw)
	if object == nil {
		return nil
	}
	block := &ledgerBlock{kind: ledgerString(object["type"]), raw: ledgerCloneRaw(raw)}
	switch block.kind {
	case "text", "reasoning":
		block.text = ledgerString(object["text"])
	case "tool-call":
		block.callID = ledgerString(object["id"])
		if block.callID == "" {
			block.callID = ledgerString(object["callId"])
		}
		block.name = ledgerString(object["name"])
		block.arguments = ledgerRawString(ledgerRawMessage(object["arguments"]))
	}
	return block
}

// ledgerStreamFirstToken reads both raw and compact AssistantStreamRecord forms
// (dsh-llm/src/assistant-stream.ts:20-44, 327-341).
func ledgerStreamFirstToken(stream []json.RawMessage) *int64 {
	for _, raw := range stream {
		record := ledgerObjectRaw(raw)
		if record == nil {
			continue
		}
		typ := ledgerString(record["type"])
		if typ == "chunk" {
			chunk := ledgerObject(record["chunk"])
			if ledgerChunkIsToken(chunk) {
				if at := ledgerPositiveTime(ledgerInt64(record["time"])); at != nil {
					return at
				}
			}
			continue
		}
		time0 := ledgerInt64(record["time0"])
		dt := ledgerAnyArray(record["dt"])
		switch typ {
		case "text-chunks", "reasoning-chunks":
			for i, value := range ledgerAnyArray(record["texts"]) {
				at := time0 + ledgerRunDelta(dt, i)
				if text, ok := value.(string); ok && text != "" {
					return ledgerPositiveTime(at)
				}
			}
		case "tool-call-chunks":
			args := ledgerAnyArray(record["args"])
			name, hasName := record["name"].(string)
			for i := range args {
				at := time0 + ledgerRunDelta(dt, i)
				if (hasName && name != "") || ledgerString(args[i]) != "" {
					return ledgerPositiveTime(at)
				}
			}
		}
	}
	return nil
}

func ledgerRunDelta(dt []any, index int) int64 {
	var sum int64
	for i := 0; i < index && i < len(dt); i++ {
		sum += ledgerInt64(dt[i])
	}
	return sum
}

func ledgerChunkIsToken(chunk map[string]any) bool {
	switch ledgerString(chunk["type"]) {
	case "text-delta", "reasoning-delta":
		return ledgerString(chunk["text"]) != ""
	case "tool-call-delta":
		return ledgerString(chunk["argumentsDelta"]) != "" || ledgerString(chunk["name"]) != ""
	default:
		return false
	}
}

func ledgerParseUsage(raw json.RawMessage) *ledgerUsage {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil
	}
	var wire struct {
		Input      *int64 `json:"inputTokens"`
		CacheRead  *int64 `json:"cacheReadTokens"`
		CacheWrite *int64 `json:"cacheWriteTokens"`
		Output     *int64 `json:"outputTokens"`
		Reasoning  *int64 `json:"reasoningTokens"`
	}
	if json.Unmarshal(raw, &wire) != nil {
		return nil
	}
	return &ledgerUsage{Input: wire.Input, CacheRead: wire.CacheRead, CacheWrite: wire.CacheWrite, Output: wire.Output, Reasoning: wire.Reasoning}
}

func ledgerAddUsage(current, next *ledgerUsage) *ledgerUsage {
	if current == nil && next == nil {
		return nil
	}
	out := &ledgerUsage{}
	if current != nil {
		out = ledgerCloneUsage(current)
	}
	if next == nil {
		return out
	}
	out.Input = ledgerSumInt64(out.Input, next.Input)
	out.CacheRead = ledgerSumInt64(out.CacheRead, next.CacheRead)
	out.CacheWrite = ledgerSumInt64(out.CacheWrite, next.CacheWrite)
	out.Output = ledgerSumInt64(out.Output, next.Output)
	out.Reasoning = ledgerSumInt64(out.Reasoning, next.Reasoning)
	return out
}

func ledgerSumInt64(left, right *int64) *int64 {
	if left == nil && right == nil {
		return nil
	}
	var value int64
	if left != nil {
		value += *left
	}
	if right != nil {
		value += *right
	}
	return &value
}

func ledgerCloneUsage(usage *ledgerUsage) *ledgerUsage {
	if usage == nil {
		return nil
	}
	out := *usage
	return &out
}

// ledgerDisplayFailure is the AUTH-safe displayFailure projection
// (trajectory-event-projection.ts:116-137).
func ledgerDisplayFailure(raw json.RawMessage) (message, code string) {
	if len(raw) == 0 {
		return "", ""
	}
	object := ledgerObjectRaw(raw)
	if object == nil {
		var text string
		if json.Unmarshal(raw, &text) == nil {
			return text, ""
		}
		return string(raw), ""
	}
	code = ledgerString(object["code"])
	if code == "AUTH" {
		return "", code
	}
	if message = ledgerString(object["message"]); message != "" {
		return message, code
	}
	return ledgerRawJSON(object), code
}

func ledgerToolOutput(blocks []json.RawMessage, isError bool, failure json.RawMessage) string {
	text := ledgerContentText(blocks, "text", "\n")
	if text != "" {
		return text
	}
	if isError {
		object := ledgerObjectRaw(failure)
		if object != nil {
			code := ledgerString(object["code"])
			name := ledgerString(object["name"])
			if name != "" && code != "" {
				return name + ": " + code
			}
			return name + code
		}
		return "error"
	}
	if len(blocks) == 0 {
		return ""
	}
	return ledgerRawArrayJSON(blocks)
}

func ledgerContentText(blocks []json.RawMessage, kind, separator string) string {
	var parts []string
	for _, raw := range blocks {
		block := ledgerObjectRaw(raw)
		if block == nil || ledgerString(block["type"]) != kind {
			continue
		}
		if text, ok := block["text"].(string); ok {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, separator)
}

func ledgerContentPreview(blocks []json.RawMessage) string {
	for _, raw := range blocks {
		block := ledgerObjectRaw(raw)
		if block != nil && ledgerString(block["type"]) == "text" {
			if text, ok := block["text"].(string); ok {
				return text
			}
		}
	}
	return ""
}

func ledgerToolSchemas(raw json.RawMessage) map[string]json.RawMessage {
	out := map[string]json.RawMessage{}
	for _, entry := range ledgerRawArray(raw) {
		if name := ledgerString(ledgerObjectRaw(entry)["name"]); name != "" {
			out[name] = ledgerCloneRaw(entry)
		}
	}
	return out
}

func ledgerConfigRoute(raw json.RawMessage) (provider, model string) {
	config := ledgerObjectRaw(raw)
	return ledgerString(config["provider"]), ledgerString(config["model"])
}

func ledgerPromptJSON(prompt *ledgerPromptSnapshot) string {
	if prompt == nil {
		return ""
	}
	object := map[string]any{"system": prompt.System}
	if len(prompt.Config) > 0 {
		object["config"] = prompt.Config
	}
	if len(prompt.Tools) > 0 {
		object["tools"] = prompt.Tools
	}
	return ledgerRawJSON(object)
}

func ledgerFrameTime(frame apiclient.Frame) *int64 { return ledgerPositiveTime(frame.Time) }

func ledgerPositiveTime(value int64) *int64 {
	if value <= 0 {
		return nil
	}
	return &value
}

func ledgerDuration(started, completed *int64) *int64 {
	if started == nil || completed == nil {
		return nil
	}
	value := *completed - *started
	if value < 0 {
		value = 0
	}
	return &value
}

// ledgerRawJSON pretty-prints actual JSON and recursively removes AUTH failure
// messages/reasons before they can reach the Inspector or search index.
func ledgerRawJSON(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case json.RawMessage:
		if len(typed) == 0 {
			return ""
		}
		var decoded any
		if json.Unmarshal(typed, &decoded) != nil {
			return string(typed)
		}
		return ledgerRawJSON(decoded)
	case []json.RawMessage:
		return ledgerRawArrayJSON(typed)
	case string:
		raw, _ := json.Marshal(typed)
		return string(raw)
	}
	ledgerSanitizeAuth(value)
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return ""
	}
	return string(raw)
}

func ledgerRawArrayJSON(values []json.RawMessage) string {
	if values == nil {
		return ""
	}
	return ledgerRawJSON(json.RawMessage(ledgerMustMarshal(values)))
}

func ledgerSanitizeAuth(value any) {
	switch typed := value.(type) {
	case map[string]any:
		if ledgerString(typed["code"]) == "AUTH" {
			if _, ok := typed["message"]; ok {
				typed["message"] = ""
			}
			if _, ok := typed["reason"]; ok {
				typed["reason"] = ""
			}
		}
		for _, child := range typed {
			ledgerSanitizeAuth(child)
		}
	case []any:
		for _, child := range typed {
			ledgerSanitizeAuth(child)
		}
	case json.RawMessage:
		var decoded any
		if json.Unmarshal(typed, &decoded) == nil {
			ledgerSanitizeAuth(decoded)
		}
	}
}

func ledgerDecodeData(raw json.RawMessage) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var out map[string]any
	if json.Unmarshal(raw, &out) != nil || out == nil {
		return map[string]any{}
	}
	return out
}

func ledgerObject(value any) map[string]any {
	object, _ := value.(map[string]any)
	if object == nil {
		return map[string]any{}
	}
	return object
}

func ledgerObjectRaw(raw json.RawMessage) map[string]any {
	if len(raw) == 0 {
		return nil
	}
	var out map[string]any
	if json.Unmarshal(raw, &out) != nil {
		return nil
	}
	return out
}

func ledgerRawMessage(value any) json.RawMessage {
	if value == nil {
		return nil
	}
	if raw, ok := value.(json.RawMessage); ok {
		return ledgerCloneRaw(raw)
	}
	return ledgerMustMarshal(value)
}

func ledgerRawArray(value any) []json.RawMessage {
	if value == nil {
		return nil
	}
	if raw, ok := value.(json.RawMessage); ok {
		var values []json.RawMessage
		if json.Unmarshal(raw, &values) != nil {
			return nil
		}
		return values
	}
	if values, ok := value.([]json.RawMessage); ok {
		return values
	}
	raw := ledgerRawMessage(value)
	var values []json.RawMessage
	if len(raw) == 0 || json.Unmarshal(raw, &values) != nil {
		return nil
	}
	return values
}

func ledgerAnyArray(value any) []any {
	values, _ := value.([]any)
	return values
}

func ledgerCloneRaw(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), raw...)
}

func ledgerMustMarshal(value any) []byte {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return raw
}

func ledgerJSONCompact(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var out bytes.Buffer
	if json.Compact(&out, raw) != nil {
		return string(raw)
	}
	return out.String()
}

func ledgerRawString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text
	}
	return ledgerJSONCompact(raw)
}

func ledgerString(value any) string {
	text, _ := value.(string)
	return text
}

func ledgerBool(value any) bool {
	truth, _ := value.(bool)
	return truth
}

func ledgerInt(value any) int { return int(ledgerInt64(value)) }

func ledgerInt64(value any) int64 {
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case float32:
		return int64(typed)
	case int:
		return int64(typed)
	case int64:
		return typed
	case json.Number:
		value, _ := typed.Int64()
		return value
	default:
		return 0
	}
}

func ledgerInt64Pointer(value any) *int64 {
	if value == nil {
		return nil
	}
	converted := ledgerInt64(value)
	return &converted
}
