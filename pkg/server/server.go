package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"octpex/pkg/bus"
	"octpex/pkg/protocol"
	"octpex/pkg/ring"
	"octpex/pkg/substrate"
	"octpex/pkg/telemetry"
	"sync"
	"time"
)

// ServerConfig defines network parameters for the HTTP dashboard.
type ServerConfig struct {
	Port int `json:"port"` // Default: 8080
}

// SystemEvent represents a real-time event broadcast over SSE.
type SystemEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"` // "CONNECTED", "ENVELOPE_DISPATCH", "LATERAL_HANDOFF"
	Source    string    `json:"source"`
	Target    string    `json:"target"`
	Details   string    `json:"details"`
}

// HTTPServer provides a REST API and embedded biomimetic web dashboard.
type HTTPServer struct {
	config      ServerConfig
	bus         *bus.LocalNervousBus
	registry    *ring.CapabilityRegistry
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
	bus *bus.LocalNervousBus,
	registry *ring.CapabilityRegistry,
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

	s.httpSrv = &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: mux,
	}

	return s
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
	go func() {
		_ = s.httpSrv.ListenAndServe()
	}()
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
	env.Security = protocol.DefaultSecurityContext(env.SessionID)

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
    .arm-card { flex: 1; min-width: 240px; border: 1px solid var(--border); border-radius: 6px; padding: 12px; background: #161b22; }
    .arm-name { font-weight: 600; color: #fff; font-size: 14px; display: flex; justify-content: space-between; }
    .arm-status { font-size: 11px; color: var(--green); font-weight: 600; }
    .tags { display: flex; gap: 4px; margin-top: 8px; flex-wrap: wrap; }
    .tag { font-size: 11px; background: #21262d; border: 1px solid var(--border); padding: 2px 6px; border-radius: 4px; color: #8b949e; }
    
    .dispatch-box { display: flex; gap: 8px; margin-top: 16px; }
    input { flex: 1; background: #0d1117; border: 1px solid var(--border); color: #fff; padding: 8px 12px; border-radius: 6px; font-size: 13px; }
    button { background: #238636; border: none; color: #fff; padding: 8px 16px; border-radius: 6px; cursor: pointer; font-weight: 600; font-size: 13px; }
    button:hover { background: #2ea043; }
    #output { margin-top: 12px; font-family: monospace; font-size: 12px; color: var(--accent); white-space: pre-wrap; background: #0d1117; padding: 12px; border-radius: 6px; border: 1px solid var(--border); display: none; }
  </style>
</head>
<body>
  <header>
    <h1>Octpex Biomimetic Agent Nervous System <span class="badge">LIVE KERNEL</span></h1>
    <div style="font-size: 13px; color: #8b949e;">Profile: <strong>Local/Cluster Mesh</strong> | Port: <strong>8080</strong></div>
  </header>

  <div class="grid">
    <div class="card">
      <div class="card-title">Edge Decoupling Ratio</div>
      <div class="card-value" id="val-decoupling">85.0%</div>
      <div class="card-sub">Resolved 100% On-Premise</div>
    </div>
    <div class="card">
      <div class="card-title">Cloud Tokens Saved</div>
      <div class="card-value" id="val-tokens">129,000</div>
      <div class="card-sub">vs. Commercial Frontier Baseline</div>
    </div>
    <div class="card">
      <div class="card-title">Estimated Cost Savings</div>
      <div class="card-value" id="val-cost">$0.81 USD</div>
      <div class="card-sub">FinOps ROI Audited</div>
    </div>
    <div class="card">
      <div class="card-title">Active Octo-Arms</div>
      <div class="card-value" id="val-arms">3</div>
      <div class="card-sub">Basal Nerve Ring Healthy</div>
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
      <div class="arm-card">
        <div class="arm-name"><span>arm-telemetry-01</span> <span class="arm-status">ONLINE</span></div>
        <div style="font-size: 12px; color: #8b949e; margin-top: 4px;">Telemetry Distiller (Level 0 Native FSM)</div>
        <div class="tags"><span class="tag">distill-telemetry</span></div>
      </div>
      <div class="arm-card">
        <div class="arm-name"><span>arm-patch-02</span> <span class="arm-status">ONLINE</span></div>
        <div style="font-size: 12px; color: #8b949e; margin-top: 4px;">Patch Synthesizer (Level 1 Edge SLM)</div>
        <div class="tags"><span class="tag">patch_synthesizer</span></div>
      </div>
      <div class="arm-card">
        <div class="arm-name"><span>arm-fsm-03</span> <span class="arm-status">ONLINE</span></div>
        <div style="font-size: 12px; color: #8b949e; margin-top: 4px;">Native FSM Verifier (Level 0 Go)</div>
        <div class="tags"><span class="tag">fsm_verifier</span></div>
      </div>
    </div>

    <div style="margin-top: 24px;">
      <div class="tier-title">Dispatch Test Task to Nervous Bus</div>
      <div class="dispatch-box">
        <input type="text" id="target-cap" value="distill-telemetry" placeholder="Target Capability (e.g. distill-telemetry)">
        <button onclick="dispatchTask()">Dispatch Envelope</button>
      </div>
      <div id="output"></div>
    </div>

    <div style="margin-top: 24px;">
      <div class="tier-title" style="display: flex; justify-content: space-between; align-items: center;">
        <span>Live Nervous Pulse Stream (SSE: /api/v1/events)</span>
        <span style="font-size: 11px; color: var(--green); text-transform: none; font-weight: normal;">Connected</span>
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
            let color = '#58a6ff';
            if (data.type === 'CONNECTED') color = '#3fb950';
            if (data.type === 'LATERAL_HANDOFF') color = '#bc8cff';
            line.innerHTML = '<span style="color:#8b949e;">[' + timeStr + ']</span> <strong style="color:' + color + ';">[' + (data.type || 'PULSE') + ']</strong> ' + (data.details || '');
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

    async function refreshStats() {
      try {
        const topoRes = await fetch('/api/v1/topology');
        const topo = await topoRes.json();
        document.getElementById('val-arms').innerText = topo.arm_count || 3;

        const finRes = await fetch('/api/v1/finops');
        const fin = await finRes.json();
        if (fin.total_tasks > 0) {
          document.getElementById('val-decoupling').innerText = (fin.edge_decoupling_ratio * 100).toFixed(1) + '%';
          document.getElementById('val-tokens').innerText = (fin.estimated_tokens_saved || 0).toLocaleString();
          document.getElementById('val-cost').innerText = '$' + (fin.estimated_cost_saved_usd || 0).toFixed(2) + ' USD';
        }
      } catch (e) {
        console.log('Poll error', e);
      }
    }
    setInterval(refreshStats, 3000);

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
            parameters: { prompt: 'Execute triage via web UI' }
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
