# detect-angular-dashboards

Detect angular dashboards in a running Grafana instance using Grafana API.

Requires a service account, with `Plugins / Plugin Writer` permissions (or "Admin" if using OSS without RBAC).

The reason behind admin rights is that the plugins endpoint returns all plugins only if the token can view and 
install plugins.


## Usage

```bash
$ go build -v
$ GRAFANA_TOKEN=abcd ./detect-angular-dashboards [GRAFANA_URL=http://127.0.0.1:3000/api]
```
