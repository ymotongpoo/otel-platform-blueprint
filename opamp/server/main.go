// opamp-server is a minimal OpAMP server for learning the protocol.
// It is NOT a production reference: no authentication, no tenancy, no
// persistence, no HA. It serves one remote configuration file to every
// connected agent and pushes updates when the file changes.
package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/open-telemetry/opamp-go/server"
	"github.com/open-telemetry/opamp-go/server/types"
)

type agentState struct {
	conn            types.Connection
	instanceID      string
	remoteConfigOK  string
	effectiveConfig string
	healthy         bool
	lastSeen        time.Time
	// failedHash is the hash of a config this agent reported FAILED for.
	// The server must not re-push a config the agent could not apply,
	// otherwise it fights the supervisor's rollback.
	failedHash string
}

type opampServer struct {
	mu         sync.Mutex
	agents     map[string]*agentState
	configPath string
	configBody []byte
	configHash []byte
}

func newOpampServer(configPath string) *opampServer {
	return &opampServer{agents: map[string]*agentState{}, configPath: configPath}
}

func (s *opampServer) loadConfig() (changed bool, err error) {
	body, err := os.ReadFile(s.configPath)
	if err != nil {
		return false, err
	}
	sum := sha256.Sum256(body)
	s.mu.Lock()
	defer s.mu.Unlock()
	if string(s.configHash) == string(sum[:]) {
		return false, nil
	}
	s.configBody = body
	s.configHash = sum[:]
	return true, nil
}

func (s *opampServer) remoteConfig() *protobufs.AgentRemoteConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	return &protobufs.AgentRemoteConfig{
		Config: &protobufs.AgentConfigMap{
			ConfigMap: map[string]*protobufs.AgentConfigFile{
				"remote.yaml": {Body: s.configBody, ContentType: "text/yaml"},
			},
		},
		ConfigHash: s.configHash,
	}
}

func (s *opampServer) onMessage(_ context.Context, conn types.Connection, msg *protobufs.AgentToServer) *protobufs.ServerToAgent {
	uid := fmt.Sprintf("%x", msg.InstanceUid)
	s.mu.Lock()
	st, ok := s.agents[uid]
	if !ok {
		st = &agentState{conn: conn, instanceID: uid}
		s.agents[uid] = st
		log.Printf("agent connected: %s", uid)
	}
	st.lastSeen = time.Now()
	if h := msg.GetHealth(); h != nil {
		st.healthy = h.Healthy
	}
	if ec := msg.GetEffectiveConfig(); ec != nil {
		for _, f := range ec.GetConfigMap().GetConfigMap() {
			st.effectiveConfig = string(f.Body)
		}
	}
	rcs := msg.GetRemoteConfigStatus()
	if rcs != nil {
		st.remoteConfigOK = rcs.Status.String()
		if rcs.Status == protobufs.RemoteConfigStatuses_RemoteConfigStatuses_FAILED {
			st.failedHash = string(rcs.LastRemoteConfigHash)
			log.Printf("agent %s reported FAILED for config %x, will not re-push it", uid, rcs.LastRemoteConfigHash)
		}
	}
	currentHash := string(s.configHash)
	failed := st.failedHash == currentHash
	s.mu.Unlock()

	resp := &protobufs.ServerToAgent{InstanceUid: msg.InstanceUid}
	// Send the remote config on first contact and whenever the agent's
	// applied hash differs from the current one, unless the agent already
	// reported that this exact config failed to apply.
	if !failed && (rcs == nil || string(rcs.LastRemoteConfigHash) != currentHash) {
		resp.RemoteConfig = s.remoteConfig()
	}
	return resp
}

func (s *opampServer) pushToAll(ctx context.Context) {
	s.mu.Lock()
	conns := make([]types.Connection, 0, len(s.agents))
	for _, st := range s.agents {
		conns = append(conns, st.conn)
	}
	s.mu.Unlock()
	for _, c := range conns {
		msg := &protobufs.ServerToAgent{RemoteConfig: s.remoteConfig()}
		if err := c.Send(ctx, msg); err != nil {
			log.Printf("push failed: %v", err)
		}
	}
}

func (s *opampServer) statusPage(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fmt.Fprintf(w, "opamp-server status\nserving config: %s (sha256 %x)\n\n", s.configPath, s.configHash)
	ids := make([]string, 0, len(s.agents))
	for id := range s.agents {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		st := s.agents[id]
		fmt.Fprintf(w, "agent %s\n  healthy: %v\n  remote config status: %s\n  last seen: %s\n  effective config bytes: %d\n\n",
			id, st.healthy, st.remoteConfigOK, st.lastSeen.Format(time.RFC3339), len(st.effectiveConfig))
	}
}

func (s *opampServer) effectiveConfigPage(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, st := range s.agents {
		fmt.Fprintf(w, "# agent %s\n%s\n", st.instanceID, st.effectiveConfig)
	}
}

func main() {
	configPath := os.Getenv("REMOTE_CONFIG_PATH")
	if configPath == "" {
		configPath = "/etc/opamp/remote.yaml"
	}
	s := newOpampServer(configPath)
	if _, err := s.loadConfig(); err != nil {
		log.Fatalf("load remote config: %v", err)
	}

	srv := server.New(nil)
	settings := server.StartSettings{
		Settings: server.Settings{
			Callbacks: types.Callbacks{
				OnConnecting: func(*http.Request) types.ConnectionResponse {
					return types.ConnectionResponse{
						Accept: true,
						ConnectionCallbacks: types.ConnectionCallbacks{
							OnMessage: s.onMessage,
						},
					}
				},
			},
		},
		ListenEndpoint: ":4320",
		ListenPath:     "/v1/opamp",
	}
	if err := srv.Start(settings); err != nil {
		log.Fatalf("start opamp server: %v", err)
	}
	log.Printf("opamp server listening on :4320/v1/opamp, serving %s", configPath)

	// Poll the config file and push changes to connected agents.
	go func() {
		for {
			time.Sleep(2 * time.Second)
			changed, err := s.loadConfig()
			if err != nil {
				log.Printf("reload config: %v", err)
				continue
			}
			if changed {
				log.Printf("remote config changed, pushing to agents")
				s.pushToAll(context.Background())
			}
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/status", s.statusPage)
	mux.HandleFunc("/effective", s.effectiveConfigPage)
	log.Printf("status page on :4321/status")
	if err := http.ListenAndServe(":4321", mux); err != nil {
		log.Fatal(err)
	}
}
