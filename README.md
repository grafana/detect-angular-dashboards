# detect-angular-dashboards

Detect angular dashboards in a running Grafana instance using Grafana API.

Compatible with Grafana OSS, Grafana Enterprise, On-Prem and Grafana Cloud.

## Configuration

The program requires a service account and API key to work properly.

Configuration changes slightly depending on your version of Grafana.

We recommend using Grafana >= 10.1.0, otherwise Angular private plugins (that are not in Grafana's catalog) will be ignored.

### Grafana >= 10.1.0

Create a service account with `Viewer` role.

Then, create a service account token for the newly created service account and set it to the `GRAFANA_TOKEN` env var.

### Grafana < 10.1.0

> Warning, Angular private plugins will be ignored from the scan when using Grafana <= 10.1.0.

Create a service account, with `Plugins / Plugin Writer` permissions (or "Admin" if using OSS without RBAC).

The reason behind admin rights is that the plugins endpoint returns all plugins only if the token can view and install plugins.

Then, create a service account token for the newly created service account and set it to the `GRAFANA_TOKEN` env var.



## Usage

### Pre-built binaries

You can download pre-built binaries from the [releases](https://github.com/grafana/detect-angular-dashboards/releases) section.

Then, run the program:

```bash
GRAFANA_TOKEN=abcd ./detect-angular-dashboards [GRAFANA_URL=http://127.0.0.1:3000/api]
````

### Building from source

You need to install Go and [Mage](https://magefile.org/).

Then, clone the repository, build and run the program:

```bash
mage build:current
GRAFANA_TOKEN=abcd ./dist/linux_amd64/detect-angular-dashboards [GRAFANA_URL=http://127.0.0.1:3000/api]
```

### Docker image

Clone the repository and build the Docker image:

```bash
docker build -t detect-angular-dashboards .
docker run --rm -it -e GRAFANA_TOKEN=abcd detect-angular-dashboards http://172.17.0.1:3000/api
```


## LICENSE

Apache License 2.0
