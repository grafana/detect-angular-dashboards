# detect-angular-dashboards

Detect angular dashboards in a running Grafana instance using Grafana API.

## Configuration

The program requires a service account and API key to work properly.

Configuration changes slightly depending on your version of Grafana.

### Grafana >= 10.1.0

Create a service account with `Viewer` role.

Then, create a service account token for the newly created service account and set it to the `GRAFANA_TOKEN` env var.

### Grafana < 10.1.0

> Warning, Angular private plugins will be ignored from the scan when using Grafana <= 10.1.0.

Create a service account, with `Plugins / Plugin Writer` permissions (or "Admin" if using OSS without RBAC).

The reason behind admin rights is that the plugins endpoint returns all plugins only if the token can view and install plugins.

Then, create a service account token for the newly created service account and set it to the `GRAFANA_TOKEN` env var.

## Usage

```bash
$ go build -v
$ GRAFANA_TOKEN=abcd ./detect-angular-dashboards [GRAFANA_URL=http://127.0.0.1:3000/api]
```
