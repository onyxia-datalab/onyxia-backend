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

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// ========= Types =========

type InstallInput struct {
	Namespace   string         `json:"namespace"`
	ReleaseName string         `json:"releaseName"`
	Chart       string         `json:"chart"`   // ex: "bitnami/nginx" ou "oci://registry-1.docker.io/bitnamicharts/nginx"
	Version     string         `json:"version"` // optionnel (laisser vide => latest)
	RepoURL     string         `json:"repoUrl"` // requis pour "foo/bar" si pas d’alias helm (inutile pour oci://)
	Values      map[string]any `json:"values"`
}

type Event struct {
	Type string      `json:"type"` // "status" | "done"
	Data interface{} `json:"data"`
}

// ========= SSE Hub =========

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

// ========= Helpers =========

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

func selectorForRelease(release string) string {
	return labels.SelectorFromSet(labels.Set{"app.kubernetes.io/instance": release}).String()
}

// ========= main =========

func main() {
	hub := NewHub()

	// Client Kube partagé (in-cluster ou ~/.kube/config), exposant .Config() et .Clientset()
	kc, err := kube.NewClient("")
	if err != nil {
		log.Fatalf("kube client init: %v", err)
	}

	mux := http.NewServeMux()

	// PUT /install -> Helm (Wait=false) puis readiness via informers
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
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
			defer cancel()
			hub.Publish(op, Event{Type: "status", Data: map[string]any{"status": "installing"}})

			// 1) Helm install (Wait=false)
			if err := helmInstallWaitFalse(ctx, kc.Config(), hub, op, in); err != nil {
				log.Printf("[OP %s] HELM ERROR: %v", op, err)
				hub.Publish(
					op,
					Event{
						Type: "done",
						Data: map[string]any{"status": "failed", "error": err.Error()},
					},
				)
				return
			}
			hub.Publish(
				op,
				Event{
					Type: "status",
					Data: map[string]any{"status": "installed", "wait": "via-informers"},
				},
			)

			// 2) Watch readiness via informers (clientset direct de internal/kube)
			if err := waitReleaseReadyWithInformers(ctx, kc.Clientset(), in.Namespace, in.ReleaseName, hub, op); err != nil {
				log.Printf("[OP %s] READY ERROR: %v", op, err)
				hub.Publish(
					op,
					Event{
						Type: "done",
						Data: map[string]any{"status": "failed", "error": err.Error()},
					},
				)
				return
			}
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

	// GET /events/{op} -> SSE
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

		t := time.NewTicker(30 * time.Second)
		defer t.Stop()

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
			case <-t.C:
				writeSSE(w, "status", map[string]any{"status": "heartbeat"})
				flusher.Flush()
			}
		}
	})

	s := &http.Server{Addr: ":8080", Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	log.Println("Wait=false + Informers + SSE listening on :8080")
	log.Fatal(s.ListenAndServe())
}

// ========= Helm install (Wait=false) =========

func helmInstallWaitFalse(
	ctx context.Context,
	cfg *rest.Config,
	hub *Hub,
	op string,
	in InstallInput,
) error {
	// kubectl ConfigFlags comme RESTClientGetter — forcer l’usage de notre *rest.Config
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

	// Workspace par op (évite collisions et simplifie oci)
	opDir, err := opTempDir(op)
	if err != nil {
		return fmt.Errorf("op temp dir: %w", err)
	}
	defer os.RemoveAll(opDir)

	cacheDir := filepath.Join(opDir, "cache")
	repoDir := filepath.Join(opDir, "repo")
	_ = os.MkdirAll(cacheDir, 0o755)
	_ = os.MkdirAll(repoDir, 0o755)

	settings := cli.New()
	settings.RepositoryCache = cacheDir
	settings.RepositoryConfig = filepath.Join(repoDir, "repositories.yaml")

	// OCI support
	rc, err := registry.NewClient(registry.ClientOptEnableCache(true))
	if err != nil {
		return fmt.Errorf("registry client: %w", err)
	}
	helmCfg.RegistryClient = rc

	inst := action.NewInstall(helmCfg)
	inst.Namespace = in.Namespace
	inst.ReleaseName = in.ReleaseName
	inst.Wait = false               // <--- important
	inst.Timeout = 10 * time.Minute // borne l’action (apply + hooks)
	inst.CreateNamespace = false
	inst.Atomic = false

	var chartPath string
	if strings.HasPrefix(in.Chart, "oci://") {
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
		inst.ChartPathOptions.Version = in.Version
		inst.ChartPathOptions.RepoURL = in.RepoURL

		log.Printf("[OP %s] locate chart %s (repoUrl=%q version=%q)", op, in.Chart, in.RepoURL, in.Version)
		chartPath, err = inst.ChartPathOptions.LocateChart(in.Chart, settings)
		if err != nil && in.Version != "" && strings.Contains(err.Error(), "not found") {
			// fallback → latest
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

	ch, err := loader.Load(chartPath)
	if err != nil {
		return fmt.Errorf("load chart: %w", err)
	}

	log.Printf("[OP %s] helm install (Wait=false) starting…", op)
	if _, err := inst.RunWithContext(ctx, ch, in.Values); err != nil {
		return fmt.Errorf("helm install: %w", err)
	}
	log.Printf("[OP %s] helm install submitted", op)
	return nil
}

// ========= Readiness via informers =========

func waitReleaseReadyWithInformers(
	ctx context.Context,
	cs kubernetes.Interface,
	namespace, release string,
	hub *Hub,
	op string,
) error {
	selector := selectorForRelease(release)

	factory := informers.NewSharedInformerFactoryWithOptions(
		cs,
		0, // resync désactivé (event-driven)
		informers.WithNamespace(namespace),
		informers.WithTweakListOptions(func(lo *metav1.ListOptions) {
			lo.LabelSelector = selector
		}),
	)

	depInf := factory.Apps().V1().Deployments()
	stsInf := factory.Apps().V1().StatefulSets()
	dsInf := factory.Apps().V1().DaemonSets()
	podInf := factory.Core().V1().Pods()
	jobInf := factory.Batch().V1().Jobs()
	svcInf := factory.Core().V1().Services()

	type state struct {
		deploy   map[string]*appsv1.Deployment
		sts      map[string]*appsv1.StatefulSet
		ds       map[string]*appsv1.DaemonSet
		pods     map[string]*corev1.Pod
		jobs     map[string]*batchv1.Job
		services map[string]*corev1.Service
	}
	st := &state{
		deploy:   map[string]*appsv1.Deployment{},
		sts:      map[string]*appsv1.StatefulSet{},
		ds:       map[string]*appsv1.DaemonSet{},
		pods:     map[string]*corev1.Pod{},
		jobs:     map[string]*batchv1.Job{},
		services: map[string]*corev1.Service{},
	}
	mu := &sync.Mutex{}

	readyCheck := func() (bool, string) {
		mu.Lock()
		defer mu.Unlock()

		// Deployments
		for _, d := range st.deploy {
			spec := int32(1)
			if d.Spec.Replicas != nil {
				spec = *d.Spec.Replicas
			}
			if d.Status.AvailableReplicas < spec || !hasDeploymentAvailable(d) {
				return false, fmt.Sprintf(
					"deployment %s not ready (%d/%d)",
					d.Name,
					d.Status.AvailableReplicas,
					spec,
				)
			}
		}
		// StatefulSets
		for _, s := range st.sts {
			spec := int32(1)
			if s.Spec.Replicas != nil {
				spec = *s.Spec.Replicas
			}
			if s.Status.ReadyReplicas < spec {
				return false, fmt.Sprintf(
					"statefulset %s not ready (%d/%d)",
					s.Name,
					s.Status.ReadyReplicas,
					spec,
				)
			}
		}
		// DaemonSets
		for _, d := range st.ds {
			if d.Status.NumberReady < d.Status.DesiredNumberScheduled {
				return false, fmt.Sprintf(
					"daemonset %s not ready (%d/%d)",
					d.Name,
					d.Status.NumberReady,
					d.Status.DesiredNumberScheduled,
				)
			}
		}
		// Jobs
		for _, j := range st.jobs {
			for _, c := range j.Status.Conditions {
				if c.Type == batchv1.JobFailed && c.Status == corev1.ConditionTrue {
					return false, fmt.Sprintf("job %s failed", j.Name)
				}
			}
			if j.Spec.Completions != nil {
				if j.Status.Succeeded < *j.Spec.Completions {
					return false, fmt.Sprintf(
						"job %s not complete (%d/%d)",
						j.Name,
						j.Status.Succeeded,
						*j.Spec.Completions,
					)
				}
			} else if j.Status.Succeeded == 0 {
				return false, fmt.Sprintf("job %s not complete", j.Name)
			}
		}
		// Pods
		for _, p := range st.pods {
			if p.Status.Phase == corev1.PodFailed {
				return false, fmt.Sprintf("pod %s failed", p.Name)
			}
			if !isPodReady(p) {
				return false, fmt.Sprintf("pod %s not ready", p.Name)
			}
		}
		return true, "all ready"
	}

	emit := func(kind, name, action string) {
		hub.Publish(op, Event{Type: "status", Data: map[string]any{
			"kind": kind, "name": name, "action": action,
		}})
	}

	addHandlers := func() {
		depInf.Informer().AddEventHandler(simpleHandlers(
			func(obj any) {
				emit("Deployment", obj.(*appsv1.Deployment).Name, "add")
				mu.Lock()
				st.deploy[obj.(*appsv1.Deployment).Name] = obj.(*appsv1.Deployment)
				mu.Unlock()
			},
			func(_, newObj any) {
				emit("Deployment", newObj.(*appsv1.Deployment).Name, "update")
				mu.Lock()
				st.deploy[newObj.(*appsv1.Deployment).Name] = newObj.(*appsv1.Deployment)
				mu.Unlock()
			},
			func(obj any) {
				emit("Deployment", obj.(*appsv1.Deployment).Name, "delete")
				mu.Lock()
				delete(st.deploy, obj.(*appsv1.Deployment).Name)
				mu.Unlock()
			},
		))
		stsInf.Informer().AddEventHandler(simpleHandlers(
			func(obj any) {
				emit("StatefulSet", obj.(*appsv1.StatefulSet).Name, "add")
				mu.Lock()
				st.sts[obj.(*appsv1.StatefulSet).Name] = obj.(*appsv1.StatefulSet)
				mu.Unlock()
			},
			func(_, newObj any) {
				emit("StatefulSet", newObj.(*appsv1.StatefulSet).Name, "update")
				mu.Lock()
				st.sts[newObj.(*appsv1.StatefulSet).Name] = newObj.(*appsv1.StatefulSet)
				mu.Unlock()
			},
			func(obj any) {
				emit("StatefulSet", obj.(*appsv1.StatefulSet).Name, "delete")
				mu.Lock()
				delete(st.sts, obj.(*appsv1.StatefulSet).Name)
				mu.Unlock()
			},
		))
		dsInf.Informer().AddEventHandler(simpleHandlers(
			func(obj any) {
				emit("DaemonSet", obj.(*appsv1.DaemonSet).Name, "add")
				mu.Lock()
				st.ds[obj.(*appsv1.DaemonSet).Name] = obj.(*appsv1.DaemonSet)
				mu.Unlock()
			},
			func(_, newObj any) {
				emit("DaemonSet", newObj.(*appsv1.DaemonSet).Name, "update")
				mu.Lock()
				st.ds[newObj.(*appsv1.DaemonSet).Name] = newObj.(*appsv1.DaemonSet)
				mu.Unlock()
			},
			func(obj any) {
				emit("DaemonSet", obj.(*appsv1.DaemonSet).Name, "delete")
				mu.Lock()
				delete(st.ds, obj.(*appsv1.DaemonSet).Name)
				mu.Unlock()
			},
		))
		podInf.Informer().AddEventHandler(simpleHandlers(
			func(obj any) {
				emit("Pod", obj.(*corev1.Pod).Name, "add")
				mu.Lock()
				st.pods[obj.(*corev1.Pod).Name] = obj.(*corev1.Pod)
				mu.Unlock()
			},
			func(_, newObj any) {
				emit("Pod", newObj.(*corev1.Pod).Name, "update")
				mu.Lock()
				st.pods[newObj.(*corev1.Pod).Name] = newObj.(*corev1.Pod)
				mu.Unlock()
			},
			func(obj any) {
				emit("Pod", obj.(*corev1.Pod).Name, "delete")
				mu.Lock()
				delete(st.pods, obj.(*corev1.Pod).Name)
				mu.Unlock()
			},
		))
		jobInf.Informer().AddEventHandler(simpleHandlers(
			func(obj any) {
				emit("Job", obj.(*batchv1.Job).Name, "add")
				mu.Lock()
				st.jobs[obj.(*batchv1.Job).Name] = obj.(*batchv1.Job)
				mu.Unlock()
			},
			func(_, newObj any) {
				emit("Job", newObj.(*batchv1.Job).Name, "update")
				mu.Lock()
				st.jobs[newObj.(*batchv1.Job).Name] = newObj.(*batchv1.Job)
				mu.Unlock()
			},
			func(obj any) {
				emit("Job", obj.(*batchv1.Job).Name, "delete")
				mu.Lock()
				delete(st.jobs, obj.(*batchv1.Job).Name)
				mu.Unlock()
			},
		))
		svcInf.Informer().AddEventHandler(simpleHandlers(
			func(obj any) {
				emit("Service", obj.(*corev1.Service).Name, "add")
				mu.Lock()
				st.services[obj.(*corev1.Service).Name] = obj.(*corev1.Service)
				mu.Unlock()
			},
			func(_, newObj any) {
				emit("Service", newObj.(*corev1.Service).Name, "update")
				mu.Lock()
				st.services[newObj.(*corev1.Service).Name] = newObj.(*corev1.Service)
				mu.Unlock()
			},
			func(obj any) {
				emit("Service", obj.(*corev1.Service).Name, "delete")
				mu.Lock()
				delete(st.services, obj.(*corev1.Service).Name)
				mu.Unlock()
			},
		))
	}

	stop := make(chan struct{})
	defer close(stop)

	addHandlers()
	factory.Start(stop)
	for t, ok := range factory.WaitForCacheSync(ctx.Done()) {
		if !ok {
			return fmt.Errorf("cache sync failed for %v", t)
		}
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout or canceled")
		case <-ticker.C:
			ok, why := readyCheck()
			hub.Publish(op, Event{Type: "status", Data: map[string]any{"readiness": why}})
			if ok {
				return nil
			}
		}
	}
}

// ========= utils =========

func hasDeploymentAvailable(d *appsv1.Deployment) bool {
	for _, c := range d.Status.Conditions {
		if c.Type == appsv1.DeploymentAvailable && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func isPodReady(p *corev1.Pod) bool {
	if p.Status.Phase != corev1.PodRunning {
		return false
	}
	for _, cs := range p.Status.ContainerStatuses {
		if !cs.Ready {
			return false
		}
	}
	return true
}

// ResourceEventHandler minimal
type handlerSet struct {
	add    func(obj any)
	update func(oldObj, newObj any)
	del    func(obj any)
}

func simpleHandlers(add func(any), update func(any, any), del func(any)) handlerSet {
	return handlerSet{add: add, update: update, del: del}
}
func (h handlerSet) OnAdd(obj any, _ bool)       { h.add(obj) }
func (h handlerSet) OnUpdate(oldObj, newObj any) { h.update(oldObj, newObj) }
func (h handlerSet) OnDelete(obj any)            { h.del(obj) }
