```bash
go run ./poc/helm-wait.main.go


curl -fsS -X PUT http://localhost:8080/install \
 -H 'Content-Type: application/json' \
 -d "$(jq -n --argjson vals "$(yq -o=json '.' values.yaml)" '{
   namespace:"user-ddecrulle",
   releaseName:"jupyter-python-626146",
   chart:"jupyter-python",
   repoUrl:"https://inseefrlab.github.io/helm-charts-interactive-services",
   values:$vals
 }')" | tee /tmp/install.json

curl -N "http://localhost:8080$(jq -r .eventsUrl /tmp/install.json)"

```

## Helm `Wait=true` (bloquant)

- **PUT `/install`** lance `helm install` avec `Wait=true`.
- **Ce que fait Helm en interne** :
  - Applique les manifests, puis **boucle en pull** (`client-go`) avec des **GET/LIST** réguliers sur les ressources créées.
  - _Waiters_ par type :
    - **Deployment** : `AvailableReplicas == Spec.Replicas` + condition `DeploymentAvailable=True`.
    - **StatefulSet** : `ReadyReplicas >= Spec.Replicas`.
    - **Pod** : phase `Running` et condition `Ready=True`.
    - **Job** : `Complete=True`.
- **Résultat** :
  - Si tous les waiters passent avant `Timeout` → release marquée **`deployed`** dans le storage Helm (Secrets/ConfigMaps).
  - Si échec/timeout → **erreur** ; avec `Atomic=true` Helm tente un **rollback**.
- **SSE côté API** :
  - `status: installing` au démarrage.
  - `done: { "status": "deployed" }` si ok.
  - `done: { "status": "failed", "error": "..." }` sinon.
- **Timeout & blocage** :
  - `ins.Timeout` borne la boucle d’attente Helm.
  - La goroutine d’install est **occupée** jusqu’à la fin (bloquant pour l’opération).
- **Points clés / prérequis** :
  - Pas d’informers côté app.
  - RBAC suffisant pour que Helm crée/observe les ressources.
  - `HELM_DRIVER` (par défaut `secret`) doit être accessible.
- **À retenir** :
  - Simple, fiable.
  - Pull (`GET/LIST`) interne à Helm.
  - Pas de granularité temps réel fine vers le front.
