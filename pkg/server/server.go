package server

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"octpex/pkg/bus"
	"octpex/pkg/protocol"
	"slices"
	"strings"

	"octpex/pkg/substrate"
	"octpex/pkg/telemetry"
	"sync"
	"time"
)

// ServerConfig defines network parameters for the HTTP dashboard.
type ServerConfig struct {
	TLS  *tls.Config `json:"-"`
	Port int         `json:"port"` // Default: 8080
}

// SystemEvent represents a real-time event broadcast over SSE.
type SystemEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"` // "CONNECTED", "ENVELOPE_DISPATCH", "LATERAL_HANDOFF"
	Source    string    `json:"source"`
	Target    string    `json:"target"`
	Details   string    `json:"details"`
}

type Publisher interface {
	Publish(context.Context, *protocol.NervousEnvelope) (*protocol.StepResult, error)
}
type Topology interface {
	ListHealthyArms() []*protocol.ArmManifest
}

// HTTPServer provides a REST API and embedded biomimetic web dashboard.
type HTTPServer struct {
	config      ServerConfig
	bus         Publisher
	registry    Topology
	substrate   substrate.DataSubstrateSPI
	finops      *telemetry.FinOpsCollector
	httpSrv     *http.Server
	subscribers map[chan SystemEvent]bool
	eventsMu    sync.RWMutex
	mu          sync.RWMutex
}

// NewHTTPServer creates an initialized HTTP server.
func NewHTTPServer(
	cfg ServerConfig,
	bus Publisher,
	registry Topology,
	sub substrate.DataSubstrateSPI,
	finops *telemetry.FinOpsCollector,
) *HTTPServer {
	if cfg.Port <= 0 {
		cfg.Port = 8080
	}
	s := &HTTPServer{
		config:      cfg,
		bus:         bus,
		registry:    registry,
		substrate:   sub,
		finops:      finops,
		subscribers: make(map[chan SystemEvent]bool),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/v1/topology", s.handleTopology)
	mux.HandleFunc("/api/v1/finops", s.handleFinOps)
	mux.HandleFunc("/api/v1/dispatch", s.handleDispatch)
	mux.HandleFunc("/api/v1/events", s.handleEvents)
	mux.HandleFunc("/api/v1/register", s.handleRegister)
	mux.HandleFunc("/api/v1/heartbeat", s.handleHeartbeat)
	mux.HandleFunc("/api/v1/chat", s.handleChat)
	mux.HandleFunc("/api/v1/skills/tree", s.handleSkillTree)

	s.httpSrv = &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: mux,
	}

	return s
}

// ServeHTTP exposes the same routes to an embedding server or an in-process caller.
func (s *HTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.httpSrv.Handler.ServeHTTP(w, r)
}

// BroadcastEvent publishes an event to all connected SSE clients.
func (s *HTTPServer) BroadcastEvent(evt SystemEvent) {
	if evt.Timestamp.IsZero() {
		evt.Timestamp = time.Now()
	}
	s.eventsMu.RLock()
	defer s.eventsMu.RUnlock()
	for ch := range s.subscribers {
		select {
		case ch <- evt:
		default:
		}
	}
}

func (s *HTTPServer) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := make(chan SystemEvent, 16)
	s.eventsMu.Lock()
	s.subscribers[ch] = true
	s.eventsMu.Unlock()

	defer func() {
		s.eventsMu.Lock()
		delete(s.subscribers, ch)
		s.eventsMu.Unlock()
	}()

	// Send initial connect handshake
	fmt.Fprintf(w, "data: %s\n\n", `{"type":"CONNECTED","details":"Octpex real-time nervous pulse stream active"}`)
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case evt, ok := <-ch:
			if !ok {
				return
			}
			data, err := json.Marshal(evt)
			if err == nil {
				fmt.Fprintf(w, "data: %s\n\n", string(data))
				flusher.Flush()
			}
		}
	}
}

// Start begins listening on the configured HTTP port in a background goroutine.
func (s *HTTPServer) Start() error {
	ln, err := net.Listen("tcp", s.httpSrv.Addr)
	if err != nil {
		return err
	}
	if s.config.TLS != nil {
		ln = tls.NewListener(ln, s.config.TLS)
	}
	go s.httpSrv.Serve(ln)
	return nil
}

// Stop gracefully shuts down the HTTP server.
func (s *HTTPServer) Stop(ctx context.Context) error {
	return s.httpSrv.Shutdown(ctx)
}

// Shutdown is an alias for Stop to support standard server shutdown interfaces.
func (s *HTTPServer) Shutdown(ctx context.Context) error {
	return s.Stop(ctx)
}

func (s *HTTPServer) handleTopology(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var arms []*protocol.ArmManifest
	if s.registry != nil {
		arms = s.registry.ListHealthyArms()
	}

	resp := map[string]any{
		"system":      "Octpex Biomimetic Nervous System",
		"version":     "0.5.0",
		"active_arms": arms,
		"arm_count":   len(arms),
		"timestamp":   time.Now().Unix(),
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *HTTPServer) handleSkillTree(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var arms []*protocol.ArmManifest
	if s.registry != nil {
		arms = s.registry.ListHealthyArms()
	}
	tree := protocol.BuildSkillTree(arms)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(tree)
}

func (s *HTTPServer) handleFinOps(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.finops == nil {
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "no_finops_collector"})
		return
	}
	summary := s.finops.GenerateSummary(1500)
	_ = json.NewEncoder(w).Encode(summary)
}

func (s *HTTPServer) handleDispatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	peer := r.Header.Get("X-Octpex-Federation-Source")
	if peer != "" && (r.TLS == nil || len(r.TLS.VerifiedChains) == 0 || r.TLS.VerifiedChains[0][0].VerifyHostname(peer) != nil) {
		http.Error(w, "unverified federation peer", http.StatusForbidden)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var env protocol.NervousEnvelope
	if err := json.NewDecoder(r.Body).Decode(&env); err != nil {
		http.Error(w, fmt.Sprintf("invalid envelope payload: %v", err), http.StatusBadRequest)
		return
	}

	if env.EnvelopeID == "" {
		env.EnvelopeID = fmt.Sprintf("env-web-%d", time.Now().UnixNano())
	}
	if env.TraceID == "" {
		env.TraceID = fmt.Sprintf("trace-web-%d", time.Now().UnixNano())
	}
	if env.SessionID == "" {
		env.SessionID = "sess-web-ui"
	}
	if env.SourceNode == "" {
		env.SourceNode = "web-ui-operator"
	}
	if peer != "" {
		if env.Plane != protocol.PlaneLateral || env.HopCount < 1 || env.Security.MaxAllowedHops < 1 || env.Security.MaxAllowedHops > 2 || !slices.Contains(env.Security.AllowedCapabilities, env.TargetCapability) {
			http.Error(w, "invalid forwarded scope", http.StatusForbidden)
			return
		}
	} else {
		if (env.Plane != "" && env.Plane != protocol.PlaneVertical) || env.HopCount != 0 {
			http.Error(w, "expected initial vertical dispatch", http.StatusForbidden)
			return
		}
		env.Plane = protocol.PlaneVertical
		env.Security = protocol.DefaultSecurityContext(env.SessionID)
		env.Security.AllowMutation = false
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	result, err := s.bus.Publish(ctx, &env)
	s.BroadcastEvent(SystemEvent{
		Timestamp: time.Now(),
		Type:      "ENVELOPE_DISPATCH",
		Source:    env.SourceNode,
		Target:    env.TargetCapability,
		Details:   fmt.Sprintf("Dispatched %s (Session: %s, Trace: %s)", env.EnvelopeID, env.SessionID, env.TraceID),
	})

	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
		return
	}

	_ = json.NewEncoder(w).Encode(result)
}

func (s *HTTPServer) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var manifest protocol.ArmManifest
	if err := json.NewDecoder(r.Body).Decode(&manifest); err != nil {
		http.Error(w, fmt.Sprintf("invalid manifest: %v", err), http.StatusBadRequest)
		return
	}
	if manifest.ArmID == "" {
		http.Error(w, "arm_id is required", http.StatusBadRequest)
		return
	}
	if manifest.Telemetry.Status == "" {
		manifest.Telemetry.Status = "HEALTHY"
	}
	manifest.Telemetry.LastHeartbeatUnix = time.Now().Unix()

	// Register with Topology registry if registrable
	if reg, ok := s.registry.(interface {
		RegisterArm(*protocol.ArmManifest) error
		DeregisterArm(string) error
	}); ok {
		_ = reg.DeregisterArm(manifest.ArmID)
		_ = reg.RegisterArm(&manifest)
	}

	// Wire HTTP proxy forwarder on the bus for external endpoints
	if manifest.Endpoint != "" {
		if subBus, ok := s.bus.(interface {
			Subscribe(string, bus.MessageHandler)
		}); ok {
			endpoint := manifest.Endpoint
			for _, cap := range manifest.CapabilityTags {
				subBus.Subscribe(cap, func(ctx context.Context, env *protocol.NervousEnvelope) (*protocol.StepResult, error) {
					bodyBytes, _ := json.Marshal(env)
					targetURL := strings.TrimRight(endpoint, "/") + "/api/v1/dispatch"
					req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(bodyBytes))
					if err != nil {
						return nil, err
					}
					req.Header.Set("Content-Type", "application/json")
					resp, err := http.DefaultClient.Do(req)
					if err != nil {
						return nil, fmt.Errorf("failed to dispatch to arm %s: %w", endpoint, err)
					}
					defer resp.Body.Close()
					if resp.StatusCode != http.StatusOK {
						var errBody map[string]any
						_ = json.NewDecoder(resp.Body).Decode(&errBody)
						errMsg := fmt.Sprintf("arm returned HTTP %d", resp.StatusCode)
						if e, ok := errBody["error"].(string); ok && e != "" {
							errMsg += ": " + e
						}
						return nil, fmt.Errorf("%s", errMsg)
					}
					var res struct {
						Status     string               `json:"status"`
						StepResult *protocol.StepResult `json:"step_result"`
						Output     any                  `json:"output"`
						Error      string               `json:"error"`
					}
					if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
						return nil, err
					}
					if res.StepResult != nil {
						return res.StepResult, nil
					}
					outMsg, _ := json.Marshal(res.Output)
					return &protocol.StepResult{
						Status:        "SUCCESS",
						OutputMessage: string(outMsg),
					}, nil
				})
			}
		}
	}

	skillsInfo := ""
	if len(manifest.Skills) > 0 {
		skillsInfo = fmt.Sprintf(", %d registered skills", len(manifest.Skills))
	}
	s.BroadcastEvent(SystemEvent{
		Timestamp: time.Now(),
		Type:      "ARM_REGISTERED",
		Source:    manifest.ArmID,
		Target:    strings.Join(manifest.CapabilityTags, ","),
		Details:   fmt.Sprintf("Arm %s online with capabilities %v%s (Endpoint: %s)", manifest.ArmID, manifest.CapabilityTags, skillsInfo, manifest.Endpoint),
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"status": "REGISTERED", "arm_id": manifest.ArmID})
}

func (s *HTTPServer) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var hb struct {
		ArmID      string `json:"arm_id"`
		Status     string `json:"status"`
		QueueDepth int    `json:"queue_depth"`
	}
	if err := json.NewDecoder(r.Body).Decode(&hb); err != nil || hb.ArmID == "" {
		http.Error(w, "invalid heartbeat", http.StatusBadRequest)
		return
	}
	status := hb.Status
	if status == "" {
		status = "HEALTHY"
	}
	if reg, ok := s.registry.(interface {
		Heartbeat(string, int, string) error
	}); ok {
		_ = reg.Heartbeat(hb.ArmID, hb.QueueDepth, status)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"status": "ACK", "arm_id": hb.ArmID})
}

func (s *HTTPServer) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Message string `json:"message"`
		Prompt  string `json:"prompt"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	userText := strings.TrimSpace(req.Message)
	if userText == "" {
		userText = strings.TrimSpace(req.Prompt)
	}
	if userText == "" {
		http.Error(w, "message or prompt is required", http.StatusBadRequest)
		return
	}

	targetCap := "distill-telemetry"
	params := map[string]any{}
	textLower := strings.ToLower(userText)

	if strings.Contains(textLower, "weather") || strings.Contains(textLower, "temp") {
		targetCap = "get_current_temperature"
		loc := ""
		unit := "c"

		// Dynamic entity extraction: strip conversational prefixes and suffixes
		cleaned := userText
		prefixes := []string{
			"what is the weather in", "what is the temperature in", "weather in", "weather for",
			"temperature in", "temperature of", "check weather for", "check weather in",
			"tell me the weather in", "get weather for", "get weather in", "weather", "temp",
		}
		for _, prefix := range prefixes {
			if strings.HasPrefix(strings.ToLower(cleaned), prefix) {
				cleaned = strings.TrimSpace(cleaned[len(prefix):])
				break
			}
		}
		suffixes := []string{
			"weather", "temperature", "forecast", "temp",
		}
		for _, suffix := range suffixes {
			if strings.HasSuffix(strings.ToLower(cleaned), suffix) {
				cleaned = strings.TrimSpace(cleaned[:len(cleaned)-len(suffix)])
				break
			}
		}
		cleaned = strings.TrimSpace(cleaned)

		if cleaned == "" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"role":  "assistant",
				"reply": "No specific query location detected. Which city or region would you like to check? (e.g. 'Washington', 'Beijing', 'London', or 'Paris').",
			})
			return
		}

		// The location entity is passed directly to the arm, where the model-driven
		// Geographic Disambiguation skill dynamically resolves it using world knowledge.
		loc = cleaned

		if strings.Contains(textLower, "dual") {
			unit = "d"
		} else if strings.Contains(textLower, "fahrenheit") || strings.Contains(textLower, "°f") {
			unit = "f"
		}
		params["location"] = loc
		params["unit"] = unit
	} else if strings.Contains(textLower, "telemetry") {
		targetCap = "distill-telemetry"
		params["metric"] = "latency"
		params["window"] = "5m"
	} else {
		// If capability tag is typed bare
		targetCap = userText
	}

	env := protocol.NewEnvelope(
		fmt.Sprintf("env-chat-%d", time.Now().UnixNano()),
		fmt.Sprintf("trace-chat-%d", time.Now().UnixNano()),
		fmt.Sprintf("sess-chat-%d", time.Now().UnixNano()),
		protocol.PlaneVertical,
		protocol.MsgTaskDispatch,
		"octo-brain-chat",
	)
	env.TargetCapability = targetCap
	env.Parameters = params

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	result, err := s.bus.Publish(ctx, env)
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		replyText := fmt.Sprintf("Task dispatch failed: %v\n(Hint: Ensure an arm subscribing to '%s' capability is running and registered)", err, targetCap)
		if strings.Contains(err.Error(), "unreachable") || strings.Contains(err.Error(), "ConnectionError") {
			replyText = fmt.Sprintf("Meteorological query failed (external network unreachable):\n%v\n\nNote: Adhering to zero-trust principles, fake dummy fallbacks are rejected. For offline testing, set OCTPEX_ALLOW_OFFLINE_SIMULATION=1 to activate explicit offline simulation mode.", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"role":  "assistant",
			"reply": replyText,
			"error": err.Error(),
		})
		return
	}

	replyText := fmt.Sprintf("Execution succeeded (Status: %s). Output:\n%s", result.Status, result.OutputMessage)
	if targetCap == "get_current_temperature" {
		var wData struct {
			Location           string  `json:"location"`
			QueryTarget        string  `json:"query_target"`
			CanonicalName      string  `json:"canonical_name"`
			ProvinceOrState    string  `json:"province_or_state"`
			Country            string  `json:"country"`
			DisambiguationNote string  `json:"disambiguation_note"`
			Display            string  `json:"display"`
			Condition          string  `json:"condition"`
			Humidity           int     `json:"humidity"`
			WindKMH            float64 `json:"wind_kmh"`
			IsSimulated        bool    `json:"is_simulated"`
			DataSource         string  `json:"data_source"`
		}
		if json.Unmarshal([]byte(result.OutputMessage), &wData) == nil && (wData.Location != "" || wData.CanonicalName != "") {
			targetName := wData.Location
			if wData.CanonicalName != "" {
				targetName = wData.CanonicalName
			}
			adminHierarchy := ""
			if wData.ProvinceOrState != "" && wData.Country != "" {
				adminHierarchy = fmt.Sprintf("\n- Administrative Hierarchy: %s | Country: %s", wData.ProvinceOrState, wData.Country)
			}
			disambig := ""
			if wData.DisambiguationNote != "" {
				disambig = fmt.Sprintf("\n- Normalization & Disambiguation: %s", wData.DisambiguationNote)
			}
			simNote := ""
			if wData.IsSimulated {
				simNote = "\n- Data Origin: [Offline Simulation - No External Connectivity]"
			}
			replyText = fmt.Sprintf("Current meteorological observations for [%s]:%s%s\n- Temperature: %s\n- Condition: %s\n- Relative Humidity: %d%% | Wind Speed: %.1f km/h%s",
				targetName, adminHierarchy, disambig, wData.Display, wData.Condition, wData.Humidity, wData.WindKMH, simNote)
		}
	}

	_ = json.NewEncoder(w).Encode(map[string]any{
		"role":                  "assistant",
		"reply":                 replyText,
		"dispatched_capability": targetCap,
		"envelope_id":           env.EnvelopeID,
		"step_result":           result,
	})
}

func (s *HTTPServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(dashboardHTML))
}

const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>Octpex - Biomimetic Agent Nervous System Dashboard</title>
  <style>
    :root {
      --bg: #0d1117;
      --card-bg: #161b22;
      --border: #30363d;
      --text: #c9d1d9;
      --accent: #58a6ff;
      --green: #3fb950;
      --purple: #bc8cff;
      --orange: #f0883e;
      --btn-blue: #238636;
      --btn-hover: #2ea043;
    }
    * { box-sizing: border-box; margin: 0; padding: 0; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; }
    body { background-color: var(--bg); color: var(--text); padding: 24px; }
    header { display: flex; justify-content: space-between; align-items: center; border-bottom: 1px solid var(--border); padding-bottom: 16px; margin-bottom: 24px; }
    h1 { font-size: 20px; font-weight: 600; color: #fff; display: flex; align-items: center; gap: 8px; }
    .badge { background: #238636; color: #fff; font-size: 11px; padding: 2px 8px; border-radius: 12px; font-weight: bold; }
    .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); gap: 16px; margin-bottom: 24px; }
    .card { background: var(--card-bg); border: 1px solid var(--border); border-radius: 8px; padding: 16px; }
    .card-title { font-size: 13px; color: #8b949e; text-transform: uppercase; margin-bottom: 8px; }
    .card-value { font-size: 24px; font-weight: 700; color: #fff; }
    .card-sub { font-size: 12px; color: var(--green); margin-top: 4px; }
    
    .topology { background: var(--card-bg); border: 1px solid var(--border); border-radius: 8px; padding: 24px; margin-bottom: 24px; }
    .tier-title { font-size: 14px; font-weight: 600; color: var(--accent); margin-bottom: 12px; text-transform: uppercase; letter-spacing: 0.5px; }
    .box { border: 1px solid var(--border); border-radius: 6px; padding: 12px; margin-bottom: 16px; background: #0d1117; }
    .arms-container { display: flex; gap: 16px; overflow-x: auto; padding-top: 8px; }
    .arm-card { flex: 1; min-width: 280px; border: 1px solid var(--border); border-radius: 6px; padding: 14px; background: #161b22; display: flex; flex-direction: column; gap: 8px; }
    .arm-header { display: flex; justify-content: space-between; align-items: center; }
    .arm-name { font-weight: 600; color: #fff; font-size: 14px; }
    .arm-status { font-size: 11px; color: var(--green); font-weight: 600; display: flex; align-items: center; gap: 4px; }
    .arm-status::before { content: ""; display: inline-block; width: 8px; height: 8px; border-radius: 50%; background: var(--green); }
    .arm-role { font-size: 12px; color: #8b949e; }
    .arm-endpoint { font-size: 11px; color: var(--purple); font-family: monospace; }
    .tags { display: flex; gap: 6px; margin-top: 4px; flex-wrap: wrap; }
    .tag { font-size: 11px; background: #21262d; border: 1px solid var(--border); padding: 2px 8px; border-radius: 4px; color: var(--accent); font-family: monospace; }
    .empty-arms { color: #8b949e; font-size: 13px; font-style: italic; padding: 16px; border: 1px dashed var(--border); border-radius: 6px; width: 100%; text-align: center; }

    /* Biomimetic Dialogue Panel */
    .chat-panel { background: var(--card-bg); border: 1px solid var(--border); border-radius: 8px; padding: 20px; margin-bottom: 24px; }
    .chat-history { background: #0d1117; border: 1px solid var(--border); border-radius: 6px; height: 320px; overflow-y: auto; padding: 16px; display: flex; flex-direction: column; gap: 12px; }
    .msg { max-width: 85%; padding: 10px 14px; border-radius: 8px; font-size: 13px; line-height: 1.5; white-space: pre-wrap; word-break: break-word; }
    .msg-user { align-self: flex-end; background: #1f6feb; color: #fff; border-bottom-right-radius: 2px; }
    .msg-assistant { align-self: flex-start; background: #21262d; border: 1px solid var(--border); color: #c9d1d9; border-bottom-left-radius: 2px; }
    .msg-meta { font-size: 11px; color: #8b949e; margin-top: 4px; }
    .chat-chips { display: flex; gap: 8px; margin-top: 12px; flex-wrap: wrap; }
    .chip { background: #21262d; border: 1px solid var(--border); color: #8b949e; font-size: 12px; padding: 4px 10px; border-radius: 14px; cursor: pointer; transition: all 0.2s; }
    .chip:hover { color: #fff; border-color: var(--accent); background: #30363d; }
    .chat-input-bar { display: flex; gap: 8px; margin-top: 12px; }
    input[type="text"] { flex: 1; background: #0d1117; border: 1px solid var(--border); color: #fff; padding: 10px 14px; border-radius: 6px; font-size: 13px; outline: none; }
    input[type="text"]:focus { border-color: var(--accent); }
    button { background: var(--btn-blue); border: none; color: #fff; padding: 10px 20px; border-radius: 6px; cursor: pointer; font-weight: 600; font-size: 13px; transition: background 0.2s; }
    button:hover { background: var(--btn-hover); }
    button:disabled { opacity: 0.6; cursor: not-allowed; }

    .dispatch-box { display: flex; gap: 8px; margin-top: 16px; }
    #output { margin-top: 12px; font-family: monospace; font-size: 12px; color: var(--accent); white-space: pre-wrap; background: #0d1117; padding: 12px; border-radius: 6px; border: 1px solid var(--border); display: none; }
  </style>
</head>
<body>
  <header>
    <h1>Octpex Biomimetic Agent Nervous System <span class="badge">LIVE KERNEL</span></h1>
    <div style="font-size: 13px; color: #8b949e;">Multi-Tier Autonomous Runtime Topology</div>
  </header>

  <div class="grid">
    <div class="card">
      <div class="card-title">Edge Decoupling Ratio</div>
      <div class="card-value" id="val-decoupling">0%</div>
      <div class="card-sub">Share of completed and escalated tasks</div>
    </div>
    <div class="card">
      <div class="card-title">Estimated Cloud Tokens Saved</div>
      <div class="card-value" id="val-tokens">0</div>
      <div class="card-sub">vs. Commercial Frontier Baseline</div>
    </div>
    <div class="card">
      <div class="card-title">Estimated Cost Savings</div>
      <div class="card-value" id="val-cost">$0.00 USD</div>
      <div class="card-sub">Estimated against 1,500 cloud tokens per resolved task</div>
    </div>
    <div class="card">
      <div class="card-title">Active Octo-Arms</div>
      <div class="card-value" id="val-arms">0</div>
      <div class="card-sub">Workers with a current heartbeat</div>
    </div>
  </div>

  <!-- Biomimetic Dialogue Panel (Chat Box) -->
  <div class="chat-panel">
    <div class="tier-title" style="display: flex; justify-content: space-between; align-items: center;">
      <span>Interactive Biomimetic Dialogue / Task Delegation</span>
      <span style="font-size: 12px; color: #8b949e; text-transform: none; font-weight: normal;">Route: POST /api/v1/chat</span>
    </div>
    <div class="chat-history" id="chat-history">
      <div class="msg msg-assistant">
        Hello, I am Octpex Distributed Agent.
        <div class="msg-meta">System Ready • Central Brain Listening</div>
      </div>
    </div>
    <div class="chat-chips">
      <span class="chip" onclick="fillAndSend('Weather in Beijing')">Beijing Temperature</span>
      <span class="chip" onclick="fillAndSend('Weather in Tokyo in dual unit')">Tokyo Weather (Dual)</span>
      <span class="chip" onclick="fillAndSend('Weather in Paris')">Paris Weather</span>
      <span class="chip" onclick="fillAndSend('Weather in Washington')">Washington Weather</span>
      <span class="chip" onclick="fillAndSend('distill-telemetry')">Telemetry (distill-telemetry)</span>
    </div>
    <div class="chat-input-bar">
      <input type="text" id="chat-input" placeholder="Enter natural language command (e.g. 'Weather in Beijing' or 'distill-telemetry')..." onkeydown="if(event.key==='Enter') sendChatMessage()">
      <button id="chat-send-btn" onclick="sendChatMessage()">Send</button>
    </div>
  </div>

  <div class="topology">
    <div class="tier-title">1. OCTO-BRAIN (Central Frontier Reasoning & Global Orchestrator)</div>
    <div class="box">
      <div style="font-size: 13px;"><strong>Frontier Model:</strong> OpenAI/Gemini/Claude REST Provider | <strong>Arbitration:</strong> Dynamic MCP Synthesizer & HITL Ready</div>
    </div>

    <div class="tier-title">2. BASAL NERVE RING (Shared Data Substrate & Tool Registry)</div>
    <div class="box">
      <div style="font-size: 13px;"><strong>Data Substrate:</strong> sub:// (Atomic CAS Versioning) | <strong>Dynamic Tool Box:</strong> AbloEx Pre-Admission AST Scanned</div>
    </div>

    <div class="tier-title">3. OCTO-ARMS MESH (Horizontal Tentacles & Edge Ganglia)</div>
    <div class="arms-container" id="arms-mesh">
      <div class="empty-arms">Discovering registered arms...</div>
    </div>

    <div style="margin-top: 24px;">
      <div class="tier-title">Dispatch Raw Envelope to Nervous Bus</div>
      <div class="dispatch-box">
        <input type="text" id="target-cap" value="get_current_temperature" placeholder="Target Capability (e.g. get_current_temperature)">
        <button onclick="dispatchTask()">Dispatch Raw Envelope</button>
      </div>
      <div id="output"></div>
    </div>

    <div style="margin-top: 24px;">
      <div class="tier-title" style="display: flex; justify-content: space-between; align-items: center;">
        <span>Live Nervous Pulse Stream (SSE: /api/v1/events)</span>
        <span style="font-size: 11px; color: var(--green); text-transform: none; font-weight: normal;">SSE STREAM</span>
      </div>
      <div id="event-feed" style="background: #0d1117; border: 1px solid var(--border); border-radius: 6px; padding: 12px; height: 160px; overflow-y: auto; font-family: monospace; font-size: 11px; display: flex; flex-direction: column; gap: 4px;">
        <div style="color: #8b949e;">[System] Listening for real-time nervous envelope pulses...</div>
      </div>
    </div>
  </div>

  <script>
    // 1. Real-time Server-Sent Events (SSE) Listener
    try {
      const evtSource = new EventSource('/api/v1/events');
      evtSource.onmessage = function(e) {
        try {
          const data = JSON.parse(e.data);
          const feed = document.getElementById('event-feed');
          if (feed) {
            const line = document.createElement('div');
            const timeStr = new Date(data.timestamp || Date.now()).toLocaleTimeString();
            line.textContent = '[' + timeStr + '] ' + (data.type || 'PULSE') + ' ' + (data.details || '');
            feed.prepend(line);
            if (feed.childNodes.length > 50) {
              feed.removeChild(feed.lastChild);
            }
          }
        } catch (err) {}
      };
    } catch (e) {
      console.log('SSE connection error', e);
    }

    // 2. Topology and FinOps Polling
    async function refreshStats() {
      try {
        const topoRes = await fetch('/api/v1/topology');
        const topo = await topoRes.json();
        document.getElementById('val-arms').innerText = topo.arm_count ?? 0;

        const mesh = document.getElementById('arms-mesh');
        if (!topo.active_arms || topo.active_arms.length === 0) {
          mesh.innerHTML = '<div class="empty-arms">No external arms registered. Run <code>python examples/weather_mcp_arm/server.py 8081</code> in a terminal to register.</div>';
        } else {
          mesh.replaceChildren(...topo.active_arms.map(arm => {
            const card = document.createElement('div');
            card.className = 'arm-card';
            
            const header = document.createElement('div');
            header.className = 'arm-header';
            header.innerHTML = '<span class="arm-name">' + (arm.arm_id || 'Unknown Arm') + '</span><span class="arm-status">' + (arm.telemetry && arm.telemetry.status ? arm.telemetry.status : 'HEALTHY') + '</span>';
            
            const role = document.createElement('div');
            role.className = 'arm-role';
            role.innerText = arm.role_name || 'Edge Ganglion Worker';

            const ep = document.createElement('div');
            ep.className = 'arm-endpoint';
            ep.innerText = 'Endpoint: ' + (arm.endpoint || 'Internal Bus');

            const tags = document.createElement('div');
            tags.className = 'tags';
            (arm.capability_tags || []).forEach(t => {
              const tag = document.createElement('span');
              tag.className = 'tag';
              tag.innerText = t;
              tags.appendChild(tag);
            });

            if (arm.skills && arm.skills.length > 0) {
              arm.skills.forEach(sk => {
                const sTag = document.createElement('span');
                sTag.className = 'tag';
                sTag.style.borderColor = 'var(--purple)';
                sTag.style.color = 'var(--purple)';
                sTag.innerText = 'skill: ' + (sk.name || sk.id);
                tags.appendChild(sTag);
              });
            }

            card.appendChild(header);
            card.appendChild(role);
            card.appendChild(ep);
            card.appendChild(tags);
            return card;
          }));
        }

        const finRes = await fetch('/api/v1/finops');
        const fin = await finRes.json();
        if (typeof fin.total_tasks === "number") {
          document.getElementById('val-decoupling').innerText = (fin.edge_decoupling_ratio * 100).toFixed(1) + '%';
          document.getElementById('val-tokens').innerText = (fin.estimated_tokens_saved || 0).toLocaleString();
          document.getElementById('val-cost').innerText = '$' + (fin.estimated_cost_saved_usd || 0).toFixed(2) + ' USD';
        }
      } catch (e) {
        console.log('Poll error', e);
      }
    }
    refreshStats();
    setInterval(refreshStats, 3000);

    // 3. Interactive Dialogue (Chat) handler
    function fillAndSend(text) {
      document.getElementById('chat-input').value = text;
      sendChatMessage();
    }

    async function sendChatMessage() {
      const input = document.getElementById('chat-input');
      const text = input.value.trim();
      if (!text) return;

      const history = document.getElementById('chat-history');
      const sendBtn = document.getElementById('chat-send-btn');

      // Append user bubble
      const userMsg = document.createElement('div');
      userMsg.className = 'msg msg-user';
      userMsg.textContent = text;
      history.appendChild(userMsg);
      input.value = '';
      sendBtn.disabled = true;
      sendBtn.innerText = 'Executing...';
      history.scrollTop = history.scrollHeight;

      // Append temporary loading assistant bubble
      const assistantMsg = document.createElement('div');
      assistantMsg.className = 'msg msg-assistant';
      assistantMsg.innerHTML = '<span style="color:#8b949e;">Task dispatching over nervous bus and executing on arm...</span>';
      history.appendChild(assistantMsg);
      history.scrollTop = history.scrollHeight;

      try {
        const res = await fetch('/api/v1/chat', {
          method: 'POST',
          headers: {'Content-Type': 'application/json'},
          body: JSON.stringify({ message: text })
        });
        const data = await res.json();
        const reply = data.reply || (data.step_result ? data.step_result.output_message : JSON.stringify(data));
        const meta = data.dispatched_capability ? ('Dispatched to: ' + data.dispatched_capability + ' (Envelope: ' + (data.envelope_id || '') + ')') : '';

        assistantMsg.innerHTML = reply.replace(/\n/g, '<br>') + (meta ? '<div class="msg-meta">' + meta + '</div>' : '');
      } catch (err) {
        assistantMsg.innerHTML = '<span style="color:#f85149;">Request failed: ' + err + '</span>';
      } finally {
        sendBtn.disabled = false;
        sendBtn.innerText = 'Send';
        history.scrollTop = history.scrollHeight;
      }
    }

    // 4. Raw Envelope Dispatcher
    async function dispatchTask() {
      const cap = document.getElementById('target-cap').value;
      const out = document.getElementById('output');
      out.style.display = 'block';
      out.innerText = 'Dispatching envelope to ' + cap + '...';

      try {
        const res = await fetch('/api/v1/dispatch', {
          method: 'POST',
          headers: {'Content-Type': 'application/json'},
          body: JSON.stringify({
            plane: 'VERTICAL',
            target_capability: cap,
            parameters: { location: 'Beijing', unit: 'c', log: 'INFO ready\nWARN queue delay' }
          })
        });
        const data = await res.json();
        out.innerText = JSON.stringify(data, null, 2);
      } catch (err) {
        out.innerText = 'Dispatch failed: ' + err;
      }
    }
  </script>
</body>
</html>`
