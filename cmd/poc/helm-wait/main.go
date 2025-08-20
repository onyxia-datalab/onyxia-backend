package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	kube "github.com/onyxia-datalab/onyxia-backend/internal/kube"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/registry"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
)

// ---------- Types ----------
type InstallInput struct {
	Namespace   string         `json:"namespace"`
	ReleaseName string         `json:"releaseName"`
	Chart       string         `json:"chart"`   // ex: "bitnami/nginx" ou "oci://registry-1.docker.io/bitnamicharts/nginx"
	Version     string         `json:"version"` // optionnel (laisser vide => latest)
	RepoURL     string         `json:"repoUrl"` // requis pour "foo/bar" si pas d'alias helm (pas utilisé pour oci://)
	Values      map[string]any `json:"values"`
}

type Event struct {
	Type string      `json:"type"` // "status" | "done"
	Data interface{} `json:"data"`
}

// ---------- SSE Hub minimal ----------
type Hub struct {
	mu   sync.RWMutex
	subs map[string]map[chan Event]struct{}
}

func NewHub() *Hub { return &Hub{subs: map[string]map[chan Event]struct{}{}} }
func (h *Hub) Subscribe(op string) chan Event {
	h.mu.Lock()
	defer h.mu.Unlock()
	ch := make(chan Event, 16)
	if h.subs[op] == nil {
		h.subs[op] = map[chan Event]struct{}{}
	}
	h.subs[op][ch] = struct{}{}
	return ch
}
func (h *Hub) Unsubscribe(op string, ch chan Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if set := h.subs[op]; set != nil {
		delete(set, ch)
		if len(set) == 0 {
			delete(h.subs, op)
		}
	}
	close(ch)
}
func (h *Hub) Publish(op string, ev Event) {
	h.mu.RLock()
	subs := h.subs[op]
	h.mu.RUnlock()
	for ch := range subs {
		select {
		case ch <- ev:
		default:
		}
	}
}

func writeSSE(w http.ResponseWriter, event string, data any) {
	_, _ = w.Write([]byte("event: " + event + "\n"))
	b, _ := json.Marshal(data)
	_, _ = w.Write([]byte("data: " + string(b) + "\n\n"))
}

// ---------- Helpers ----------
func opTempDir(op string) (string, error) {
	dir := filepath.Join(os.TempDir(), "helmops", op)
	return dir, os.MkdirAll(dir, 0o755)
}

func findChartDir(root string) (string, error) {
	var hit string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if _, e := os.Stat(filepath.Join(p, "Chart.yaml")); e == nil {
				hit = p
				return filepath.SkipDir
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if hit == "" {
		return "", fmt.Errorf("chart dir not found under %s", root)
	}
	return hit, nil
}

// ---------- main ----------
func main() {
	hub := NewHub()

	// Client Kube partagé (in-cluster ou ~/.kube/config)
	kc, err := kube.NewClient("")
	if err != nil {
		log.Fatalf("kube client init: %v", err)
	}

	mux := http.NewServeMux()

	// PUT /install : lance helm install (Wait=true) en tâche de fond et notifie via SSE
	mux.HandleFunc("/install", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var in InstallInput
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if in.Namespace == "" {
			in.Namespace = "default"
		}

		op := fmt.Sprintf("op_%d", rand.Int())
		log.Printf("[OP %s] install ns=%s release=%s chart=%s version=%s repoUrl=%s",
			op, in.Namespace, in.ReleaseName, in.Chart, in.Version, in.RepoURL)

		go func() {
			t0 := time.Now()
			hub.Publish(op, Event{Type: "status", Data: map[string]any{"status": "installing"}})

			if err := helmInstallWaitTrue(context.Background(), kc.Config(), hub, op, in); err != nil {
				log.Printf(
					"[OP %s] ERROR after %s: %v",
					op,
					time.Since(t0).Round(time.Millisecond),
					err,
				)
				hub.Publish(
					op,
					Event{
						Type: "done",
						Data: map[string]any{"status": "failed", "error": err.Error()},
					},
				)
				return
			}
			log.Printf("[OP %s] OK after %s", op, time.Since(t0).Round(time.Millisecond))
			hub.Publish(op, Event{Type: "done", Data: map[string]any{
				"status": "deployed",
				"result": map[string]string{
					"releaseName": in.ReleaseName,
					"namespace":   in.Namespace,
				},
			}})
		}()

		w.Header().Set("Location", "/events/"+op)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).
			Encode(map[string]any{"operationId": op, "eventsUrl": "/events/" + op})
	})

	// GET /events/{op} : flux SSE
	mux.HandleFunc("/events/", func(w http.ResponseWriter, r *http.Request) {
		op := strings.TrimPrefix(r.URL.Path, "/events/")
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "stream unsupported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		ch := hub.Subscribe(op)
		defer hub.Unsubscribe(op, ch)

		writeSSE(w, "status", map[string]any{"status": "connected"})
		flusher.Flush()

		tick := time.NewTicker(30 * time.Second)
		defer tick.Stop()

		for {
			select {
			case <-r.Context().Done():
				return
			case ev := <-ch:
				writeSSE(w, ev.Type, ev.Data)
				flusher.Flush()
				if ev.Type == "done" {
					return
				}
			case <-tick.C:
				writeSSE(w, "status", map[string]any{"status": "heartbeat"})
				flusher.Flush()
			}
		}
	})

	s := &http.Server{Addr: ":8080", Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	log.Println("Wait=true + SSE listening on :8080")
	log.Fatal(s.ListenAndServe())
}

// ---------- Helm install (Wait=true), sans restGetter custom ----------
func helmInstallWaitTrue(
	ctx context.Context,
	cfg *rest.Config,
	hub *Hub,
	op string,
	in InstallInput,
) error {
	// kubectl ConfigFlags comme RESTClientGetter — on force l'usage de notre *rest.Config
	flags := genericclioptions.NewConfigFlags(true)
	flags.Namespace = &in.Namespace
	flags.WrapConfigFn = func(_ *rest.Config) *rest.Config { return cfg }

	helmCfg := new(action.Configuration)
	helmCfg.Log = func(format string, v ...any) {
		msg := fmt.Sprintf(format, v...)
		log.Printf("[OP %s] [HELM] %s", op, msg)
		hub.Publish(op, Event{Type: "status", Data: map[string]any{"log": msg}})
	}
	if err := helmCfg.Init(flags, in.Namespace, os.Getenv("HELM_DRIVER"), log.Printf); err != nil {
		return fmt.Errorf("helm init: %w", err)
	}

	// Espace par opération (évite les collisions /tmp)
	opDir, err := opTempDir(op)
	if err != nil {
		return fmt.Errorf("op temp dir: %w", err)
	}
	defer os.RemoveAll(opDir)

	cacheDir := filepath.Join(opDir, "cache")
	repoDir := filepath.Join(opDir, "repo")
	_ = os.MkdirAll(cacheDir, 0o755)
	_ = os.MkdirAll(repoDir, 0o755)

	// Settings Helm pour cache/config de repo
	settings := cli.New()
	settings.RepositoryCache = cacheDir
	settings.RepositoryConfig = filepath.Join(repoDir, "repositories.yaml")

	// Client OCI (pour charts oci://…)
	rc, err := registry.NewClient(registry.ClientOptEnableCache(true))
	if err != nil {
		return fmt.Errorf("registry client: %w", err)
	}
	helmCfg.RegistryClient = rc

	inst := action.NewInstall(helmCfg)
	inst.Namespace = in.Namespace
	inst.ReleaseName = in.ReleaseName
	inst.Wait = true
	inst.Timeout = 10 * time.Minute
	inst.CreateNamespace = false
	inst.Atomic = false

	var chartPath string

	if strings.HasPrefix(in.Chart, "oci://") {
		// Pull+untar dans opDir
		pull := action.NewPullWithOpts(action.WithConfig(helmCfg))
		pull.Settings = settings
		pull.Untar = true
		pull.DestDir = opDir
		pull.Version = in.Version

		log.Printf(
			"[OP %s] pull OCI chart %s (version=%q) into %s",
			op,
			in.Chart,
			in.Version,
			opDir,
		)
		if _, err := pull.Run(in.Chart); err != nil {
			return fmt.Errorf("pull oci chart: %w", err)
		}
		if chartPath, err = findChartDir(opDir); err != nil {
			return err
		}
	} else {
		// Repo HTTP
		inst.ChartPathOptions.Version = in.Version
		inst.ChartPathOptions.RepoURL = in.RepoURL

		log.Printf("[OP %s] locate chart %s (repoUrl=%q version=%q)", op, in.Chart, in.RepoURL, in.Version)
		chartPath, err = inst.ChartPathOptions.LocateChart(in.Chart, settings)
		if err != nil && in.Version != "" && strings.Contains(err.Error(), "not found") {
			// Fallback → latest
			log.Printf("[OP %s] requested version %q not found, retry latest", op, in.Version)
			hub.Publish(op, Event{Type: "status", Data: map[string]any{
				"status":           "retrying-locate",
				"reason":           "version not found; falling back to latest",
				"requestedVersion": in.Version,
			}})
			inst.ChartPathOptions.Version = ""
			chartPath, err = inst.ChartPathOptions.LocateChart(in.Chart, settings)
		}
		if err != nil {
			return fmt.Errorf("locate chart: %w", err)
		}
	}

	// Charge et installe
	log.Printf("[OP %s] load chart from %s", op, chartPath)
	ch, err := loader.Load(chartPath)
	if err != nil {
		return fmt.Errorf("load chart: %w", err)
	}

	log.Printf("[OP %s] helm install (Wait=true) starting…", op)
	if _, err := inst.RunWithContext(ctx, ch, in.Values); err != nil {
		return fmt.Errorf("helm install: %w", err)
	}
	log.Printf("[OP %s] helm install completed (release=%s)", op, in.ReleaseName)
	return nil
}
